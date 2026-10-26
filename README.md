# Warehouse

Dashboard manajemen item, lokasi, penerimaan stok, dan transfer antar lokasi dengan React + Go. Satu repository, dua aplikasi, satu kontrak OpenAPI. UI dan API berbahasa Inggris; panduan ini berbahasa Indonesia.

## Daftar isi

- [Jalankan dengan Docker](#jalankan-dengan-docker)
- [Development lokal](#development-lokal)
- [Arsitektur](#arsitektur)
  - [Model, handler, service, dan repository per fitur](#model-handler-service-dan-repository-per-fitur)
  - [Alur pembacaan lokasi dan kategori](#alur-pembacaan-lokasi-dan-kategori)
  - [Alur transfer stok](#alur-transfer-stok)
  - [Mengapa memilih monorepo](#mengapa-memilih-monorepo)
  - [Mengapa memakai Turborepo](#mengapa-memakai-turborepo)
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
    Proxy --> Router

    subgraph Backend["Backend Go"]
        Router["HTTP router<br/>Middleware dan envelope bersama"]
        ItemHandler["ItemHandler"]
        LocationHandler["LocationHandler"]
        StockHandler["StockHandler"]
        ItemService["ItemService<br/>Item dan kategori"]
        LocationService["LocationService<br/>Lokasi"]
        StockService["StockService<br/>Saldo, receive, transfer"]
        ItemRepository["ItemRepository / pgx"]
        LocationRepository["LocationRepository / pgx"]
        StockRepository["StockRepository / pgx"]
        Router --> ItemHandler --> ItemService --> ItemRepository
        Router --> LocationHandler --> LocationService --> LocationRepository
        Router --> StockHandler --> StockService --> StockRepository
    end

    ItemService <-->|"Cache kategori"| Redis[("Redis")]
    LocationService <-->|"Cache lokasi"| Redis
    ItemRepository <-->|"Item dan kategori"| PostgreSQL[("PostgreSQL")]
    LocationRepository <-->|"Lokasi"| PostgreSQL
    StockRepository <-->|"Saldo, audit, idempotensi"| PostgreSQL
```

- `backend/cmd`: entrypoint API, migration, dan seed.
- `backend/internal`: model domain, handler HTTP, service business rules, dan repository SQL.
- `backend/migrations`: migration up/down yang di-embed ke binary.
- `backend/docs/openapi.yaml`: kontrak OpenAPI 3.0, di-embed dan disajikan API.
- `frontend/src`: komponen, halaman, custom hooks, API client, dan tipe hasil generate.
- Root: npm workspaces, Turborepo, Compose, script verifikasi, dan GitHub Actions.

Tidak ada shared package kosong. Backend memiliki `package.json` hanya untuk integrasi task Turbo; proses runtime tetap Go. Build Go memakai `CGO_ENABLED=0` dalam container dan berjalan sebagai user non-root. Frontend memakai Nginx non-root dan fallback SPA untuk deep link.

### Model, handler, service, dan repository per fitur

Setiap fitur memiliki model data, handler HTTP, service untuk aturan bisnis, dan repository untuk akses PostgreSQL:

| Model                                           | Handler           | Service           | Repository           | Tanggung jawab                      |
| ----------------------------------------------- | ----------------- | ----------------- | -------------------- | ----------------------------------- |
| `Item`, `ItemInput`, `Filter`                   | `ItemHandler`     | `ItemService`     | `ItemRepository`     | CRUD item dan kategori              |
| `Location`, `LocationInput`                     | `LocationHandler` | `LocationService` | `LocationRepository` | Daftar/tambah/edit lokasi           |
| `Stock`, `StockSummary`, input dan hasil mutasi | `StockHandler`    | `StockService`    | `StockRepository`    | Saldo, receive, transfer, dan audit |

File `items.go`, `locations.go`, dan `stock.go` tersedia di masing-masing folder `internal/model/`, `internal/httpapi/`, `internal/service/`, dan `internal/repository/`. Model item dan lokasi menyediakan fungsi normalisasi dan validasi metadata yang dipanggil service. Handler menangani decoding JSON, header, UUID pada path, dan parsing parameter query; service menjalankan normalisasi dan validasi bisnis satu kali sebelum mengakses repository. Batas pagination/filter serta aturan kuantitas dan key idempotensi juga berada di service. Constraint database tetap menjaga integritas saat transaksi berjalan. Error domain berada di `internal/model/errors.go`, sedangkan parsing dan format kuantitas eksak berada di `internal/model/quantity.go`.

`cmd/api/main.go` membuat ketiga repository menggunakan pool koneksi yang sama, memasangkan setiap repository ke service yang sesuai, lalu memasangkan setiap service ke handler fitur tersebut. Interface repository didefinisikan pada service yang menggunakannya.

Helper cache di `internal/service/cache.go` digunakan oleh service item dan lokasi. Helper error PostgreSQL dan rollback transaksi berada di `internal/repository/postgres.go`. `StockRepository` melakukan pemeriksaan item/lokasi yang diperlukan dalam transaksi stok, sehingga perubahan saldo, audit, dan idempotensi tetap di-commit bersama.

Route tetap terpusat di `internal/httpapi/http.go`, bersama middleware, decoding request, dan envelope response. Struct `API` merangkai ketiga handler. Contoh alur daftar item: `GET /api/v1/items` → `ItemHandler.List()` → `ItemService.List()` → `ItemRepository.List()`. Data hasil query menggunakan `model.Item`.

### Alur pembacaan lokasi dan kategori

```mermaid
sequenceDiagram
    autonumber
    participant Browser
    participant API as LocationHandler / ItemHandler
    participant Service as LocationService / ItemService
    participant Redis
    participant Repo as LocationRepository / ItemRepository
    participant DB as PostgreSQL

    Browser->>API: GET /api/v1/locations atau /api/v1/categories
    API->>Service: LocationService.List() atau ItemService.Categories()
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
    participant API as StockHandler
    participant Service as StockService
    participant Repo as StockRepository
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

Monorepo juga sangat mendukung workflow coding dengan bantuan AI karena konteks frontend dan backend tersedia dalam satu workspace. AI dapat menelusuri hubungan antara kontrak OpenAPI, implementasi Go, tipe TypeScript, penggunaan API di UI, dan test tanpa berpindah repository atau mengandalkan salinan konteks manual. Perubahan lintas aplikasi lebih mudah dikerjakan dan ditinjau bersama, sehingga konsistensi nama field, format payload, validasi, dan penanganan error lebih mudah dijaga. Konsistensi hasilnya tetap diverifikasi melalui lint, typecheck, serta unit, integration, dan E2E test.

Untuk pengembangan dan evaluasi technical test, satu checkout menyediakan README, konfigurasi environment, Docker Compose, dan CI yang sama. Backend dan frontend tetap memiliki dependency, build, serta container masing-masing; dependency Go dikelola melalui Go modules, sedangkan dependency JavaScript melalui npm workspaces.

Trade-off-nya adalah cakupan repository dan CI menjadi lebih luas. Konfigurasi bersama dapat memicu pemeriksaan kedua aplikasi, dan pengaturan akses source code berlaku pada repository yang sama. Pilihan ini sesuai dengan proyek yang frontend dan backend-nya berkembang bersama serta dikelola oleh tim yang sama.

### Mengapa memakai Turborepo

npm workspaces mendaftarkan `backend/` dan `frontend/` sebagai workspace dan mengelola dependency JavaScript. Turborepo mengoordinasikan task di workspace tersebut. Dengan demikian, perintah seperti `npm run dev`, `npm run lint`, `npm run test`, dan `npm run build` dari root menjalankan script aplikasi yang relevan, dengan task independen dapat berjalan paralel. Script backend meneruskan pekerjaan ke toolchain Go.

Turborepo juga menyediakan cache lokal untuk task deterministik. Hasil build, lint, typecheck, atau unit test dapat digunakan kembali ketika input yang diperhitungkan tidak berubah. `turbo.json` memasukkan kontrak OpenAPI, manifest Go, dan konfigurasi terkait sebagai dependency cache; `scripts/tasks.mjs` menambahkan OS, arsitektur, serta versi Node dan Go ke cache key. Output build yang disimpan adalah `dist/**` untuk frontend dan `bin/**` untuk backend.

Task yang bergantung pada proses atau layanan aktif tetap dijalankan: `dev` dan integration test tidak dicache. Migration serta E2E dijalankan langsung melalui script masing-masing. Konfigurasi ini menjaga agar hasil cache tidak menggantikan pemeriksaan terhadap kondisi database, Redis, atau browser yang sebenarnya.

Trade-off-nya adalah tambahan dependency Turborepo dan konfigurasi cache yang perlu dirawat. Workflow dari root membutuhkan Node/npm, termasuk saat mengoordinasikan task Go. Untuk dua aplikasi, manfaat utamanya adalah perintah kerja yang konsisten dan pengulangan task lokal yang lebih cepat. Remote cache belum dikonfigurasi, sehingga reuse cache Turborepo antar-run CI belum tersedia.

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

Frontend: Vitest + Testing Library + MSW menguji debounce/fokus, validasi form, error API, state submit, pemulihan key/payload receipt dan transfer setelah remount, storage gagal/rusak, serta konfigurasi proxy dari `.env.local`. Schema quantity dipakai bersama oleh form receive dan transfer. Playwright menggunakan aplikasi dan PostgreSQL asli pada viewport desktop dan tablet, termasuk audit axe, navigasi keyboard, serta skenario commit berhasil tetapi respons diputus lalu halaman di-reload sebelum retry.

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
- Request ID dibuat oleh middleware dan dibawa melalui context/logger ke log error, cache, serta transaksi stok setelah commit. Respons timeout tetap memakai envelope JSON dan `Content-Type: application/json`.
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

Sekitar **95% kode proyek ini ditulis dengan bantuan AI** melalui OpenAI Codex, menggunakan model **GPT 6.1 Sol** dan **GPT 6 Astra**. Bantuan AI mencakup perencanaan arsitektur, scaffolding, implementasi backend/frontend, kontrak OpenAPI, penulisan test, konfigurasi Docker/CI, dan dokumentasi.

Validasi kode yang dihasilkan AI diperketat melalui **unit test, integration test, end-to-end (E2E) test, dan smoke test**. Pengujian mencakup aturan bisnis, integrasi dengan PostgreSQL dan Redis, alur pengguna di browser, serta perilaku aplikasi saat service direstart atau tidak tersedia.

**Lint pada Go dan TypeScript, serta typecheck**, menjadi bagian dari pemeriksaan kualitas untuk membantu memvalidasi kode yang dihasilkan AI. Pemeriksaan ini membantu mendeteksi masalah penggunaan tipe, pola kode yang bermasalah, dan ketidakkonsistenan implementasi, melengkapi pengujian perilaku aplikasi.

## Hasil verifikasi lokal

Diverifikasi pada 26–27 Oktober 2026 di macOS ARM64 dengan PostgreSQL 18 melalui Docker:

- Lint Go/TypeScript, typecheck, build, validasi OpenAPI, serta format check lulus.
- Unit/integration test Go dengan race detector lulus, termasuk transaksi dan concurrency transfer, korelasi request ID, envelope timeout JSON, serta timestamp UTC dan replay legacy saat diuji dengan `TZ=Asia/Jakarta`.
- **17 test frontend** dan **14 skenario Playwright** (desktop/tablet) lulus, termasuk audit axe, fokus keyboard, debounce, pagination setelah delete, receipt `0.100 + 0.200`, tambah/edit lokasi, transfer eksak tanpa perubahan total stok, serta recovery receipt/transfer setelah respons commit hilang dan halaman di-reload.
- Startup Compose, persistensi setelah restart database/API, dan failure migration lulus.
- Redis: cache hit/miss, TTL, revision yang menolak stale fill, invalidasi, serta fallback saat Redis tidak tersedia diuji otomatis.
- `govulncheck` pada dependency Go saat ini tidak menemukan kerentanan. Dua temuan moderate npm terkait React Router v6 dijelaskan di atas.
- Workflow GitHub Actions tersedia; run CI remote belum terjadi karena repository belum dipublikasikan.
