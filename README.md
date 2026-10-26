# Warehouse

Dashboard manajemen item, lokasi, penerimaan stok, dan transfer antar lokasi dengan React + Go. Satu repository, dua aplikasi, satu kontrak OpenAPI. UI dan API berbahasa Inggris; panduan ini berbahasa Indonesia.

## Daftar isi

- [Jalankan dengan Docker](#jalankan-dengan-docker)
- [Development lokal](#development-lokal)
- [Arsitektur](#arsitektur)
  - [Alur pembacaan lokasi dan kategori](#alur-pembacaan-lokasi-dan-kategori)
  - [Alur transfer stok](#alur-transfer-stok)
  - [Mengapa memilih monorepo](#mengapa-memilih-monorepo)
  - [Mengapa memakai Turborepo](#mengapa-memakai-turborepo)
- [Kontrak dan aturan bisnis](#kontrak-dan-aturan-bisnis)
  - [Transaksi dan retry](#transaksi-dan-retry)
  - [Pencarian dan filter kategori](#pencarian-dan-filter-kategori)
  - [Menggunakan Locations dan transfer](#menggunakan-locations-dan-transfer)
- [Redis](#redis)
- [Trade-off implementasi](#trade-off-implementasi)
- [Pengujian dan quality gates](#pengujian-dan-quality-gates)
- [Operasional dan batas deployment](#operasional-dan-batas-deployment)
- [Deklarasi penggunaan AI](#deklarasi-penggunaan-ai)
- [Hasil verifikasi lokal](#hasil-verifikasi-lokal)

## Jalankan dengan Docker

Prasyarat: Docker Engine/Desktop aktif dengan Docker Compose v2. Tidak perlu menginstal Go atau Node untuk mode ini.

```sh
docker compose up --build
```

- Dashboard: http://localhost:8088
- Swagger UI: http://localhost:8088/docs
- OpenAPI: http://localhost:8088/docs/openapi.yaml
- Readiness: http://localhost:8088/health/ready

Compose menjalankan PostgreSQL dan Redis, menunggu PostgreSQL sehat, menjalankan migration, mengisi tiga lokasi secara idempotent, lalu menjalankan API dan frontend. Database dimulai dengan **daftar item kosong**. Untuk contoh inventori:

```sh
docker compose --profile demo run --rm demo
```

Default credential di Compose khusus development lokal. Port hanya di-bind ke loopback. Salin `.env.example` menjadi `.env` untuk menyesuaikan port dan credential sebelum database pertama kali dibuat. Perubahan credential environment tidak otomatis mengganti password pada volume yang sudah ada. Jangan gunakan konfigurasi lokal sebagai konfigurasi produksi.

```sh
docker compose down       # container berhenti; data tetap ada
# docker compose down -v  # DESTRUKTIF: menghapus data lokal; hanya untuk reset yang disengaja
```

## Development lokal

Gunakan Node 24 (lihat `.nvmrc`), npm 11.19.1, dan Go 1.26.10. Base image Docker dipin dengan digest manifest. Versi dependency dipin dalam manifest, `package-lock.json`, dan `go.sum`.

```sh
npm install --global npm@11.19.1
npm ci
docker compose up -d db redis
cp backend/.env.example backend/.env
set -a
. backend/.env
set +a
npm run db:migrate
npm run db:seed
npm run dev
```

Vite berjalan di http://localhost:5173 dan mem-proxy `/api`, `/health`, serta `/docs` ke API port 8080. Jalankan API dari root dengan `npm run dev -w @warehouse/backend` bila frontend dijalankan terpisah. File `.env` backend harus diekspor ke shell seperti contoh di atas; Go tidak memuat dotenv secara implisit. `frontend/.env.local` dapat mengubah `API_PROXY_TARGET`.

Frontend juga dapat dijalankan dari foldernya sesuai brief:

```sh
cd frontend
npm install
npm run dev
```

Hentikan service backend/frontend Compose sebelum memakai port API 8080 untuk development lokal. Database Compose dapat tetap berjalan. Untuk data demo pada mode lokal: `npm run db:demo`.

## Arsitektur

```mermaid
flowchart LR
    Browser["Browser / React"] --> Proxy["Vite proxy (development)<br/>Nginx (Docker)"]
    Proxy --> Handler

    subgraph Backend["Backend Go"]
        Handler["HTTP handler<br/>Decode request, validasi transport, envelope"]
        Service["Service<br/>Validasi aturan bisnis dan koordinasi cache"]
        Repository["Repository / pgx<br/>SQL dan transaksi"]
        Handler --> Service --> Repository
    end

    Service <-->|"Cache lokasi dan kategori"| Redis[("Redis")]
    Repository <-->|"Data utama, stok, audit, idempotensi"| PostgreSQL[("PostgreSQL")]
```

- `backend/cmd`: entrypoint API, migration, dan seed.
- `backend/internal`: model domain, handler HTTP, service business rules, dan repository SQL.
- `backend/migrations`: migration up/down yang di-embed ke binary.
- `backend/docs/openapi.yaml`: kontrak OpenAPI 3.0, di-embed dan disajikan API.
- `frontend/src`: komponen, halaman, custom hooks, API client, dan tipe hasil generate.
- Root: npm workspaces, Turborepo, Compose, script verifikasi, dan GitHub Actions.

Tidak ada shared package kosong. Backend memiliki `package.json` hanya untuk integrasi task Turbo; proses runtime tetap Go. Build Go memakai `CGO_ENABLED=0` dalam container dan berjalan sebagai user non-root. Frontend memakai Nginx non-root dan fallback SPA untuk deep link.

### Alur pembacaan lokasi dan kategori

```mermaid
sequenceDiagram
    autonumber
    participant Browser
    participant API as HTTP handler
    participant Service
    participant Redis
    participant Repo as Repository
    participant DB as PostgreSQL

    Browser->>API: GET /api/v1/locations atau /api/v1/categories
    API->>Service: Locations() atau Categories()
    opt Cache diaktifkan
        Service->>Redis: Baca lookup dan revision
        Redis-->>Service: Data cache, miss, atau error
    end
    alt Cache hit dengan JSON valid
        Service->>Service: Decode data cache
    else Cache miss, rusak, tidak aktif, atau tidak tersedia
        Service->>Repo: Ambil daftar dari database
        Repo->>DB: SELECT lokasi atau kategori aktif
        DB-->>Repo: Hasil query
        Repo-->>Service: Daftar lokasi atau kategori
        opt Revision tersedia dari pembacaan cache
            Service->>Redis: Isi cache jika revision masih sama, dengan TTL
            Redis-->>Service: Tersimpan, revision berubah, atau error
        end
    end
    Service-->>API: Data lookup
    API-->>Browser: 200 dengan JSON envelope
```

### Alur transfer stok

```mermaid
sequenceDiagram
    autonumber
    participant Browser
    participant API as HTTP handler
    participant Service
    participant Repo as Repository
    participant DB as PostgreSQL

    Browser->>API: POST /api/v1/stock/transfer
    API->>Service: Payload dan Idempotency-Key opsional
    Service->>Service: Validasi dan normalisasi input
    Service->>Repo: Transfer(payload, key)
    Repo->>DB: BEGIN
    opt Idempotency-Key tersedia
        Repo->>DB: Advisory lock key dan baca hasil sebelumnya
        DB-->>Repo: Record idempotensi atau belum ada
    end
    alt Key dan payload cocok dengan operasi sebelumnya
        Repo->>DB: COMMIT
        Repo-->>Service: Hasil sebelumnya, replay = true
    else Key sudah dipakai dengan payload atau operasi berbeda
        Repo->>DB: ROLLBACK
        Repo-->>Service: Konflik 409
    else Operasi baru
        Repo->>DB: Kunci item aktif dengan SELECT FOR UPDATE
        Repo->>DB: Periksa lokasi, kurangi saldo asal, tambah saldo tujuan
        alt Seluruh perubahan valid
            Repo->>DB: INSERT stock_movements untuk transfer
            opt Idempotency-Key tersedia
                Repo->>DB: Simpan payload hash dan hasil idempotensi
            end
            Repo->>DB: COMMIT
            Repo-->>Service: Transfer dan saldo setelah commit
        else Item atau lokasi tidak tersedia, saldo kurang, overflow, atau error
            Repo->>DB: ROLLBACK seluruh perubahan
            Repo-->>Service: Error sesuai penyebab
        end
    end
    Service-->>API: Hasil transfer, replay, atau error
    API-->>Browser: JSON envelope dan status sesuai hasil
```

### Mengapa memilih monorepo

Backend dan frontend mengerjakan satu alur bisnis yang saling terkait. Contohnya, penambahan transfer stok membutuhkan perubahan migration, validasi Go, kontrak OpenAPI, tipe TypeScript, dialog UI, dan test. Menyimpannya dalam satu repository memungkinkan seluruh perubahan tersebut ditinjau dalam satu pull request dan dicatat dalam satu commit, sehingga hubungan antara API dan penggunaannya di frontend mudah dilacak.

Kontrak OpenAPI juga berada dekat dengan kedua aplikasi. Setelah kontrak berubah, tipe frontend dapat di-generate dan diverifikasi dalam checkout yang sama. Ini mengurangi koordinasi versi kontrak antar-repository. Kesesuaian implementasi tetap diperiksa melalui typecheck, contract test, dan E2E.

Untuk pengembangan dan evaluasi technical test, satu checkout menyediakan README, konfigurasi environment, Docker Compose, dan CI yang sama. Backend dan frontend tetap memiliki dependency, build, serta container masing-masing; dependency Go dikelola melalui Go modules, sedangkan dependency JavaScript melalui npm workspaces.

Trade-off-nya adalah cakupan repository dan CI menjadi lebih luas. Konfigurasi bersama dapat memicu pemeriksaan kedua aplikasi, dan pengaturan akses source code berlaku pada repository yang sama. Pilihan ini sesuai dengan proyek yang frontend dan backend-nya berkembang bersama serta dikelola oleh tim yang sama.

### Mengapa memakai Turborepo

npm workspaces mendaftarkan `backend/` dan `frontend/` sebagai workspace dan mengelola dependency JavaScript. Turborepo mengoordinasikan task di workspace tersebut. Dengan demikian, perintah seperti `npm run dev`, `npm run lint`, `npm run test`, dan `npm run build` dari root menjalankan script aplikasi yang relevan, dengan task independen dapat berjalan paralel. Script backend meneruskan pekerjaan ke toolchain Go.

Turborepo juga menyediakan cache lokal untuk task deterministik. Hasil build, lint, typecheck, atau unit test dapat digunakan kembali ketika input yang diperhitungkan tidak berubah. `turbo.json` memasukkan kontrak OpenAPI, manifest Go, dan konfigurasi terkait sebagai dependency cache; `scripts/tasks.mjs` menambahkan OS, arsitektur, serta versi Node dan Go ke cache key. Output build yang disimpan adalah `dist/**` untuk frontend dan `bin/**` untuk backend.

Task yang bergantung pada proses atau layanan aktif tetap dijalankan: `dev` dan integration test tidak dicache. Migration serta E2E dijalankan langsung melalui script masing-masing. Konfigurasi ini menjaga agar hasil cache tidak menggantikan pemeriksaan terhadap kondisi database, Redis, atau browser yang sebenarnya.

Trade-off-nya adalah tambahan dependency Turborepo dan konfigurasi cache yang perlu dirawat. Workflow dari root membutuhkan Node/npm, termasuk saat mengoordinasikan task Go. Untuk dua aplikasi, manfaat utamanya adalah perintah kerja yang konsisten dan pengulangan task lokal yang lebih cepat. Remote cache belum dikonfigurasi, sehingga reuse cache Turborepo antar-run CI belum tersedia.

## Kontrak dan aturan bisnis

| Method             | Endpoint                 | Fungsi                              |
| ------------------ | ------------------------ | ----------------------------------- |
| POST / GET         | `/api/v1/items`          | Tambah / list item aktif            |
| GET / PUT / DELETE | `/api/v1/items/:id`      | Detail / update penuh / soft delete |
| POST               | `/api/v1/stock/receive`  | Penerimaan stok ke satu lokasi      |
| GET                | `/api/v1/stock/:item_id` | Saldo per lokasi dan total          |
| GET / POST         | `/api/v1/locations`      | Daftar / tambah lokasi              |
| PUT                | `/api/v1/locations/:id`  | Edit metadata lokasi                |
| POST               | `/api/v1/stock/transfer` | Transfer stok antar lokasi          |
| GET                | `/api/v1/categories`     | Kategori unik seluruh item aktif    |

Envelope sukses berisi `success`, `message`, `data`; list item menambahkan `meta: { page, limit, total }`. Error berisi `success: false`, `message`, dan `errors: [{ field, reason }]`. Status utama: 400 input invalid, 404 resource tidak tersedia, 409 konflik, 500 kegagalan internal, dan 503 service unavailable/timeout. Detail database tidak dikirim ke client.

- SKU di-trim dan di-uppercase; unik juga terhadap item terhapus. Tidak ada reuse SKU atau restore.
- `search` mencocokkan substring nama/SKU secara case-insensitive. `%` dan `_` adalah karakter literal. `category` adalah exact match.
- Pagination default `page=1&limit=10`, limit maksimum 100 dan page maksimum 1.000.000. Urutan `created_at DESC, id DESC`; count dan hasil list berasal dari snapshot transaksi yang sama.
- Soft delete selalu diperbolehkan, termasuk item dengan stok. Stok dan riwayat tetap tersimpan, tetapi GET/update/delete/receive/transfer berikutnya untuk item tersebut menghasilkan 404.
- Kategori/unit merupakan metadata bebas. Mengubah unit **tidak mengonversi saldo**; UI menjelaskan hal ini pada form edit.
- Qty adalah **string desimal**, maksimal 15 digit bilangan bulat dan 3 digit pecahan. Gunakan titik, misalnya `"1.250"`; JSON number, notasi eksponen, nilai negatif, nol untuk receipt/transfer, serta presisi berlebih ditolak.
- Saldo per lokasi memakai `NUMERIC(18,3)`; total merupakan penjumlahan eksak dan bisa lebih besar dari batas satu lokasi. Backend memakai thousandths berbasis integer untuk validasi/normalisasi, frontend tidak memakai Number untuk kuantitas.
- Kode lokasi di-trim dan di-uppercase, unik tanpa membedakan kapital. Zone dan type wajib. Edit lokasi mempertahankan ID, saldo, serta riwayat yang mereferensikannya. Seed tidak menimpa perubahan metadata. Penghapusan lokasi belum disediakan.
- Timestamp dikirim UTC; UI menampilkan tanggal dalam zona waktu browser.

### Transaksi dan retry

Penerimaan mengunci baris item aktif, memverifikasi lokasi, melakukan atomic upsert saldo, menulis `stock_movements`, lalu menyimpan hasil idempotensi dalam **satu transaksi**. Soft delete memakai row lock yang sama. Jika penerimaan menang lebih dulu, stoknya tersimpan lalu item boleh dihapus; jika delete menang lebih dulu, receive ditolak. Error menyebabkan rollback.

Transfer memakai penguncian item yang sama dengan receive dan delete. Lokasi asal dan tujuan harus berbeda; jumlah harus positif dan tidak melebihi saldo asal. Debit, kredit, audit `stock_movements` (`kind=transfer`, `from_location_id`, `location_id` sebagai tujuan), dan hasil idempotensi di-commit bersama. Total item tidak berubah. Saldo kurang atau overflow di tujuan menghasilkan 409 dan rollback penuh; lokasi/item tidak ditemukan menghasilkan 404. Migration `000002` menambahkan fitur ini tanpa menghapus data lama; rollback migration tersebut ditolak bila riwayat transfer sudah ada.

`Idempotency-Key` opsional untuk API dan selalu dikirim UI. Advisory transaction lock menserialisasi penggunaan key, termasuk saat key belum tersimpan. Payload dikanonisasi sebelum di-hash. Key dan payload yang sama mengembalikan hasil operasi asli; payload atau jenis operasi berbeda menghasilkan 409. Namespace key dibagi antara receive dan transfer. Replay yang sudah berhasil tetap dapat dikembalikan setelah item dihapus karena tidak membuat mutasi baru. Key tidak kedaluwarsa dalam versi ini; retensi perlu dirancang sebelum penggunaan berskala besar.

UI tidak melakukan retry mutation otomatis. Setelah network error/5xx, retry dengan payload sama memakai key yang sama. Mengubah payload memulai operasi baru. Jangan reload halaman selama hasil receipt/transfer belum jelas: key retry hanya disimpan selama komponen masih aktif, bukan lintas reload/browser. Integrasi eksternal harus menyimpan key sampai hasil diketahui.

Contoh penerimaan:

```sh
curl http://localhost:8088/api/v1/stock/receive \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: delivery-2026-001' \
  -d '{"item_id":"<item-uuid>","location_id":"10000000-0000-4000-8000-000000000001","qty":"1.250"}'
```

### Pencarian dan filter kategori

Inventory dan Locations memakai komponen input pencarian yang sama, termasuk tinggi, warna, ikon, focus state, dan tombol clear. Pencarian item tetap memakai debounce 350 ms; pencarian lokasi menyaring daftar yang sudah dimuat.

Filter kategori berupa dropdown dengan pencarian tanpa membedakan kapital, daftar yang dapat di-scroll, serta dukungan keyboard: panah untuk navigasi, Enter untuk memilih, dan Escape untuk menutup. Pemilihan kategori mereset pagination dan disimpan di URL. Kategori terpilih tetap terlihat saat menu dibuka kembali; **All categories** menghapus filter. Menu merender maksimal 100 kategori yang cocok sekaligus agar tetap ringan, dengan petunjuk untuk mempersempit pencarian jika hasil lebih banyak. Pencarian tetap mencakup seluruh kategori aktif dari API.

### Menggunakan Locations dan transfer

Buka **Locations** di sidebar untuk mencari kode/zone/type, menambah lokasi, atau mengedit metadata. Untuk memindahkan stok, buka detail item lalu tekan ikon **Transfer stock** (panah dua arah) di panel stok. Pilih asal dan tujuan, lihat saldo asal, lalu isi jumlah. Saldo yang ditampilkan merupakan snapshot; server memeriksa kembali saldo terbaru saat transaksi berjalan.

```sh
curl http://localhost:8088/api/v1/stock/transfer \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: transfer-2026-001' \
  -d '{"item_id":"<item-uuid>","from_location_id":"10000000-0000-4000-8000-000000000001","to_location_id":"10000000-0000-4000-8000-000000000002","qty":"0.100"}'
```

## Redis

Redis 8 dipakai sebagai cache untuk `GET /api/v1/locations` dan `GET /api/v1/categories`. Backend memakai [go-redis v9](https://redis.io/docs/latest/develop/clients/go/). PostgreSQL tetap menyimpan semua data utama, transaksi stok, audit, dan idempotensi. Saldo stok dan detail item selalu dibaca dari PostgreSQL.

- `REDIS_URL` mengaktifkan cache, misalnya `redis://localhost:6379/0`; kosongkan untuk menonaktifkannya saat development lokal. Compose mengarahkannya ke service `redis`.
- `REDIS_CACHE_TTL` mengatur masa simpan, default `60s`, minimum `1ms` dan maksimum `1h`. Compose menyediakan `REDIS_PORT=6379` di loopback untuk development.
- Perubahan item melalui API menginvalidasi kategori setelah commit; tambah/edit lokasi menginvalidasi daftar lokasi. Revision acak dan conditional fill dalam Lua mencegah pembacaan yang dimulai sebelum invalidasi mengisi ulang data lama.
- Kegagalan koneksi, baca, tulis, atau invalidasi cache tidak menggagalkan operasi database. Setiap panggilan Redis dibatasi 200 ms dan tidak di-retry otomatis; warning dicatat di log. `/health/ready` tetap memeriksa PostgreSQL karena API dapat beroperasi tanpa Redis. Health Redis tersedia di `docker compose ps`.
- Jika invalidasi gagal saat Redis bermasalah, atau data diubah langsung lewat SQL/seed, lookup lama dapat terlihat sampai TTL berakhir. TTL merupakan batas konsistensi cache untuk kondisi ini. Saldo stok dan transaksi tidak bergantung pada cache.
- Cache tidak dipersistenkan; Redis dibatasi 64 MB dengan eviction `allkeys-lru`. Restart atau eviction menyebabkan cache diisi ulang dari PostgreSQL. Tidak perlu backup Redis.

```sh
docker compose exec redis redis-cli ping
docker compose exec redis redis-cli --scan --pattern 'warehouse:v1:lookups:*'
```

## Trade-off implementasi

- **Perawatan SQL manual.** Query dan mapping row ditulis langsung dengan pgx. Perubahan schema perlu diikuti pembaruan query serta mapping yang terkait; tidak ada ORM yang menyinkronkannya otomatis.
- **Kontrak Go diverifikasi lewat test.** OpenAPI menghasilkan tipe TypeScript, sedangkan handler Go ditulis manual. Pemeriksaan tipe hasil generate belum membuktikan kesesuaian response backend; contract test dan E2E tetap diperlukan.
- **Throughput dibatasi per item.** Penguncian pada baris item membuat operasi stok untuk item yang sama berjalan serial, termasuk ketika lokasinya berbeda. Jika volume transaksi per item meningkat, granularitas penguncian perlu ditinjau sambil mempertahankan konsistensi receive, transfer, dan delete.

## Pengujian dan quality gates

```sh
npm run lint
npm run typecheck
npm run test
npm run build
npm run check:api
```

Untuk integration test, buat database test terpisah satu kali:

```sh
docker compose exec -T db createdb -U warehouse warehouse_test
export TEST_DATABASE_URL='postgres://warehouse:warehouse_local@localhost:5432/warehouse_test?sslmode=disable'
export TEST_REDIS_URL='redis://localhost:6379/15'
npm run test:integration
```

Integration test membuat schema UUID terisolasi, menggunakan SQL migration sebenarnya, dan membersihkannya sesudah test. Tidak boleh menunjuk database produksi. Suite mencakup SKU duplikat, search literal/filter/pagination, soft delete dan retensi audit, receipt bersamaan, idempotensi, overflow/rollback, receive versus delete, transfer bersamaan tanpa saldo negatif, retry transfer, rollback debit/kredit/audit, transfer versus delete, serta edit lokasi dan keunikan kode. Test Redis memakai prefix UUID terisolasi tanpa `FLUSHDB`, mencakup hit/miss, TTL, revision, invalidasi, dan fallback. Test Go dijalankan dengan `-race`; lint menjalankan `go vet` dan golangci-lint.

Frontend: Vitest + Testing Library + MSW menguji debounce/fokus, validasi form, error API, state submit, dan key receipt/transfer saat retry. Playwright menggunakan aplikasi dan PostgreSQL asli pada viewport desktop dan tablet, termasuk audit axe serta navigasi keyboard.

```sh
docker compose up --build -d --wait
cd frontend
npx playwright install chromium
cd ..
npm run test:e2e
node scripts/smoke.mjs
node scripts/smoke-cache.mjs
```

`smoke-cache.mjs` memeriksa isi cache, TTL, invalidasi kategori, menghentikan Redis untuk membuktikan fallback, lalu menyalakannya kembali. `smoke.mjs` me-restart container database/backend, memastikan data tetap ada, dan menguji migration gagal untuk database yang tidak ada. Jalankan hanya pada lingkungan lokal/CI yang boleh direstart. Script mengasumsikan credential default Compose. E2E/smoke meninggalkan record yang di-soft-delete sebagai bukti retensi; tidak menghapus data secara permanen. E2E Locations juga meninggalkan lokasi berawalan `E2E-` yang terkait riwayat transfer; gunakan database lokal/test yang sesuai.

GitHub Actions menjalankan quality gates dan suite container/browser pada push/PR. Belum ada deployment otomatis atau pengiriman submission. CI baru dapat dilihat di GitHub setelah repository dipublikasikan.

## Operasional dan batas deployment

- Log JSON API mencatat request ID, method/path, status, durasi, serta receipt/transfer berhasil setelah commit. `X-Request-ID` dikembalikan ke client; body request dan credential tidak dicatat.
- `/health/live` memeriksa proses, `/health/ready` memeriksa koneksi database. Compose menahan startup API bila migration/seed gagal.
- Koneksi awal database dicoba ulang maksimal 30 detik; Nginx me-resolve ulang DNS Docker setelah restart. Request timeout 15 detik, batas JSON 1 MiB, HTTP header/read/write timeout, dan graceful shutdown 10 detik.
- Swagger UI menggunakan asset CDN yang dipin; endpoint OpenAPI tetap tersedia tanpa akses internet.
- Belum ada autentikasi, authorization, TLS publik, rate limiting, retensi audit otomatis, atau sistem alert. Target versi ini lokal/internal sesuai cakupan test; penggunaan publik memerlukan pengamanan akses dan konfigurasi operasional tambahan.
- React Router **6.30.6** dipertahankan untuk memenuhi brief. `npm audit` mencatat advisory moderate [open redirect](https://github.com/advisories/GHSA-wrjc-x8rr-h8h6) dan [SSR hydration](https://github.com/advisories/GHSA-337j-9hxr-rhxg). Aplikasi tidak memakai SSR/hydration data dan tidak menerima tujuan redirect dari input pengguna; link item dibangun dari UUID server. Ini pembatasan jalur yang relevan, bukan klaim dependency bebas kerentanan. Upgrade major dengan persetujuan perubahan requirement diperlukan untuk menutup advisory upstream. CI tetap gagal pada advisory high/critical.

Backup database lokal:

```sh
docker compose exec -T db pg_dump -U warehouse -d warehouse -Fc > warehouse.dump
```

Uji restore ke **database baru**, bukan menimpa data aktif:

```sh
docker compose exec -T db createdb -U warehouse warehouse_restore
docker compose exec -T db pg_restore -U warehouse -d warehouse_restore --no-owner < warehouse.dump
```

Simpan backup di lokasi aman di luar repository. Untuk production, jadwalkan backup, uji pemulihan, serta tentukan RPO/RTO. Migration tidak di-rollback otomatis; tinjau konsekuensi data sebelum memakai file down.

## Deklarasi penggunaan AI

Sekitar **90% kode proyek ini ditulis dengan bantuan AI** melalui OpenAI Codex, menggunakan model **GPT 6.1 Sol** dan **GPT 6 Astra**. Bantuan AI mencakup perencanaan arsitektur, scaffolding, implementasi backend/frontend, kontrak OpenAPI, penulisan test, konfigurasi Docker/CI, dan dokumentasi.

Validasi kode yang dihasilkan AI diperketat melalui **unit test, integration test, end-to-end (E2E) test, dan smoke test**. Pengujian mencakup aturan bisnis, integrasi dengan PostgreSQL dan Redis, alur pengguna di browser, serta perilaku aplikasi saat service direstart atau tidak tersedia.

**Lint pada Go dan TypeScript, serta typecheck**, menjadi bagian dari pemeriksaan kualitas untuk membantu memvalidasi kode yang dihasilkan AI. Pemeriksaan ini membantu mendeteksi masalah penggunaan tipe, pola kode yang bermasalah, dan ketidakkonsistenan implementasi, melengkapi pengujian perilaku aplikasi.

## Hasil verifikasi lokal

Diverifikasi pada 26–27 Oktober 2026 di macOS ARM64 dengan PostgreSQL 18 melalui Docker:

- Lint Go/TypeScript, typecheck, build, validasi OpenAPI, serta format check lulus.
- Unit/integration test Go dengan race detector lulus, termasuk transaksi dan concurrency transfer.
- **9 test frontend** dan **10 skenario Playwright** (desktop/tablet) lulus, termasuk audit axe, fokus keyboard, debounce, pagination setelah delete, receipt `0.100 + 0.200`, tambah/edit lokasi, dan transfer eksak tanpa perubahan total stok.
- Startup Compose, persistensi setelah restart database/API, dan failure migration lulus.
- Redis: cache hit/miss, TTL, revision yang menolak stale fill, invalidasi, serta fallback saat Redis tidak tersedia diuji otomatis.
- `govulncheck` pada dependency Go saat ini tidak menemukan kerentanan. Dua temuan moderate npm terkait React Router v6 dijelaskan di atas.
- Workflow GitHub Actions tersedia; run CI remote belum terjadi karena repository belum dipublikasikan.
