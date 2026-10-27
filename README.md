# Warehouse

Dashboard manajemen item, lokasi, penerimaan stok, dan transfer antar lokasi. Dibangun dengan **React, TypeScript, Tailwind CSS, Go, PostgreSQL, dan Redis** dalam monorepo Turborepo + npm workspaces.

UI dan API berbahasa Inggris; panduan ini berbahasa Indonesia.

## Daftar isi

- [Jalankan dengan Docker](#jalankan-dengan-docker)
- [Development lokal](#development-lokal)
- [Arsitektur](#arsitektur)
- [Keputusan teknis](#keputusan-teknis)
  - [Monorepo](#monorepo)
  - [Turborepo](#turborepo)
  - [Redis](#redis)
  - [Pemisahan handler, service, dan repository](#pemisahan-handler-service-dan-repository)
- [Pengujian](#pengujian)
- [Penggunaan AI](#penggunaan-ai)

## Jalankan dengan Docker

Prasyarat: Docker dengan Compose v2.

```sh
docker compose up --build
```

| Akses      | URL                                     |
| ---------- | --------------------------------------- |
| Dashboard  | http://localhost:8088                   |
| Swagger UI | http://localhost:8088/docs              |
| OpenAPI    | http://localhost:8088/docs/openapi.yaml |
| Readiness  | http://localhost:8088/health/ready      |

Compose menjalankan database, Redis, migration, seed lokasi, API, dan frontend. Inventori awal kosong, dengan tiga lokasi tersedia. Untuk menambahkan data contoh:

```sh
docker compose --profile demo run --rm demo
```

Data tetap tersimpan setelah `docker compose down`. Perintah `docker compose down -v` menghapus volume database.

Credential bawaan khusus development lokal. Pengaturan port dan credential tersedia di `.env.example`; perubahan credential tidak mengganti password pada volume yang sudah ada.

## Development lokal

Gunakan **Node 24**, **npm 11.19.1**, dan **Go 1.26.10**. Jika Compose penuh sedang berjalan, hentikan API/frontend terlebih dahulu dengan `docker compose stop backend frontend` agar port tidak bentrok.

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

Frontend berjalan di http://localhost:5173 dan API di http://localhost:8080. Vite mem-proxy `/api`, `/health`, dan `/docs` ke backend. Target proxy dapat diubah lewat `API_PROXY_TARGET` di `frontend/.env.local`.

Environment backend perlu diekspor seperti contoh di atas karena Go tidak memuat `.env` secara otomatis. Gunakan `npm run db:demo` untuk data contoh.

<details>
<summary>Menjalankan frontend secara terpisah</summary>

```sh
cd frontend
npm install
npm run dev
```

Jalankan backend dari root dengan `npm run dev -w @warehouse/backend`, menggunakan environment yang sudah diekspor.

</details>

## Arsitektur

```mermaid
flowchart LR
    Browser[React] --> Proxy[Vite proxy / Nginx]
    Proxy --> Handler[HTTP handler]
    Handler --> Service[Service]
    Service --> Repository[Repository / pgx]
    Repository --> PostgreSQL[(PostgreSQL)]
    Service <-->|Lokasi dan kategori| Redis[(Redis)]
```

Backend dipisahkan per fitur: **items, locations, dan stock**. Masing-masing memiliki model, handler, service, dan repository. Handler menangani transport HTTP, service memvalidasi aturan bisnis, dan repository menjalankan SQL serta transaksi.

```text
backend/
  cmd/          # Entrypoint API, migration, dan seed
  internal/
    model/      # Struktur data dan fungsi validasi
    httpapi/    # Route, handler, dan middleware
    service/    # Aturan bisnis dan interface repository
    repository/ # Query PostgreSQL dan transaksi
    cache/      # Adapter Redis
    requestlog/ # Logger per request
  migrations/   # Migration SQL
  docs/         # Kontrak OpenAPI
frontend/src/
  components/   # UI dan dialog
  pages/        # Halaman aplikasi
  hooks/        # Query, mutation, dan recovery request
  services/     # API client
  types/        # Tipe TypeScript dari OpenAPI
```

`cmd/api/main.go` merangkai dependency secara manual. Ketiga repository memakai pool PostgreSQL yang sama. Contoh alur daftar item:

`GET /api/v1/items` → `ItemHandler.List()` → `ItemService.List()` → `ItemRepository.List()`.

<details>
<summary>Sequence diagram transfer stok</summary>

```mermaid
sequenceDiagram
    participant UI as Browser
    participant API as Handler / Service
    participant Repo as StockRepository
    participant DB as PostgreSQL

    UI->>API: Transfer dan Idempotency-Key
    API->>API: Validasi input
    API->>Repo: Transfer(payload, key)
    Repo->>DB: BEGIN, kunci key, periksa hasil sebelumnya
    alt Request sudah berhasil sebelumnya
        DB-->>Repo: Hasil tersimpan
        Repo->>DB: COMMIT
    else Operasi baru
        Repo->>DB: Kunci item dan periksa saldo
        Repo->>DB: Debit, kredit, audit, dan record idempotensi
        alt Berhasil
            Repo->>DB: COMMIT
        else Gagal
            Repo->>DB: ROLLBACK
        end
    end
    Repo-->>API: Hasil atau error
    API-->>UI: JSON response
```

</details>

SQL ditulis eksplisit agar query mudah ditelusuri, dengan konsekuensi mapping dan migration perlu dirawat manual. Operasi stok dikunci per item untuk menjaga konsistensi; throughput untuk item yang sama berjalan serial.

## Keputusan teknis

### Monorepo

Backend, frontend, kontrak OpenAPI, dan test berada dalam satu repository agar perubahan lintas aplikasi dapat ditinjau dalam satu commit/PR. Ini juga mendukung workflow coding dengan AI: konteks kedua aplikasi tersedia bersama, sehingga konsistensi payload, tipe, validasi, dan UI lebih mudah dijaga. Konsekuensinya, perubahan konfigurasi bersama dapat memicu pemeriksaan kedua aplikasi.

### Turborepo

Turborepo mengoordinasikan perintah `dev`, `lint`, `typecheck`, `test`, dan `build` dari root, serta menyimpan cache task deterministik. Cache memperhitungkan kontrak API, dependency, platform, dan versi toolchain. Dev dan integration test tidak dicache; migration dan E2E dijalankan langsung. Workflow root memerlukan Node/npm, sementara runtime backend tetap Go.

### Redis

Redis dipilih untuk mengurangi query berulang pada daftar lokasi dan kategori yang sering dibaca tetapi relatif jarang berubah. Cache juga dapat dipakai bersama oleh beberapa proses API. Untuk skala technical test ini, Redis bersifat opsional dan menambah satu service yang perlu dikelola.

- Endpoint yang memakai cache: **`GET /api/v1/locations`** dan **`GET /api/v1/categories`**.
- TTL default **60 detik**, diatur melalui `REDIS_CACHE_TTL`; perubahan data melalui API menginvalidasi cache setelah commit.
- Jika Redis gagal, lookup memakai PostgreSQL. Bila invalidasi gagal, data cache lama bisa terlihat hingga TTL berakhir.
- Saldo, transaksi, audit, dan idempotensi tetap di PostgreSQL agar konsistensinya dijaga oleh transaksi database.

### Pemisahan handler, service, dan repository

Pemisahan ini membuat perubahan HTTP, aturan bisnis, dan SQL memiliki batas yang jelas. Service bergantung pada interface repository, sehingga aturan bisnis dapat diuji memakai fake repository tanpa database. Konsekuensinya jumlah file bertambah, tetapi kode per fitur lebih mudah ditelusuri dan diubah.

## Pengujian

Pemeriksaan dari root:

```sh
npm run lint
npm run typecheck
npm run test
npm run build
npm run check:api
```

Integration test memakai PostgreSQL dan Redis asli, dengan schema/key terisolasi yang dibersihkan setelah test. Buat database test satu kali:

```sh
docker compose exec -T db createdb -U warehouse warehouse_test
export TEST_DATABASE_URL='postgres://warehouse:warehouse_local@localhost:5432/warehouse_test?sslmode=disable'
export TEST_REDIS_URL='redis://localhost:6379/15'
npm run test:integration
```

Untuk E2E dan smoke test, jalankan aplikasi melalui Compose:

```sh
docker compose up --build -d --wait
cd frontend
npx playwright install chromium
cd ..
npm run test:e2e
node scripts/smoke.mjs
node scripts/smoke-cache.mjs
```

**Verifikasi lokal, 26–27 Oktober 2026:** unit/integration Go dengan race detector, **19 test frontend**, dan **14 E2E desktop/tablet** lulus. Cakupan mencakup CRUD, transaksi stok bersamaan, rollback, idempotensi setelah reload, aksesibilitas, dan fallback Redis. Lint, typecheck, build, serta validasi OpenAPI juga lulus.

Smoke test me-restart database/API dan menghentikan Redis sementara. Jalankan pada lingkungan lokal/test; pengujian meninggalkan item yang di-soft-delete dan lokasi `E2E-` sebagai bagian dari retensi riwayat.

GitHub Actions menjalankan pemeriksaan kode dan suite Docker/browser pada push/PR. Belum ada deployment otomatis.

## Penggunaan AI

Sekitar **95% kode proyek ini ditulis dengan bantuan AI** melalui OpenAI Codex, menggunakan model **GPT 6.1 Sol** dan **GPT 6 Astra**. Bantuan AI mencakup perencanaan arsitektur, scaffolding, implementasi backend/frontend, kontrak OpenAPI, penulisan test, konfigurasi Docker/CI, dan dokumentasi.

Validasi kode yang dihasilkan AI diperketat melalui **unit test, integration test, end-to-end (E2E) test, dan smoke test**. Pengujian mencakup aturan bisnis, integrasi dengan PostgreSQL dan Redis, alur pengguna di browser, serta perilaku aplikasi saat service direstart atau tidak tersedia.

**Lint pada Go dan TypeScript, serta typecheck**, menjadi bagian dari pemeriksaan kualitas untuk membantu memvalidasi kode yang dihasilkan AI. Pemeriksaan ini membantu mendeteksi masalah penggunaan tipe, pola kode yang bermasalah, dan ketidakkonsistenan implementasi, melengkapi pengujian perilaku aplikasi.
