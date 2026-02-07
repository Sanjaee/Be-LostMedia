# Zacode Go Backend

Backend server untuk aplikasi Zacode menggunakan Go dengan arsitektur clean architecture.

## Struktur Proyek

```
/yourapp
│
├── cmd/
│   └── server/
│       └── main.go
│
├── internal/
│   ├── config/          # load env, config global
│   │   └── config.go
│   │
│   ├── app/             # HTTP handler + routing (Gin)
│   │   ├── router.go
│   │   ├── auth_handler.go
│   │   ├── chat_handler.go
│   │   └── ...
│   │
│   ├── service/         # business logic / usecase
│   │   ├── auth_service.go
│   │   ├── chat_service.go
│   │   └── ...
│   │
│   ├── repository/      # DB access (gorm / raw SQL)
│   │   ├── user_repo.go
│   │   ├── chat_repo.go
│   │   └── ...
│   │
│   ├── model/           # struct model untuk DB
│   │   ├── user.go
│   │   ├── chat.go
│   │   └── ...
│   │
│   ├── websocket/       # ws hub, manager, client
│   │   ├── hub.go
│   │   ├── client.go
│   │   ├── ws_handler.go
│   │   └── ...
│   │
│   └── util/            # helper: jwt, hash, error, response
│       ├── jwt.go
│       ├── hash.go
│       └── response.go
│
├── pkg/                 # library reusable (optional)
│   └── logger/
│       └── logger.go
│
├── go.mod
├── .env
├── Dockerfile
└── docker-compose.yml
```

## Deskripsi Folder

### `cmd/server/`
Entry point aplikasi. Berisi `main.go` yang menginisialisasi dan menjalankan server.

### `internal/config/`
Konfigurasi aplikasi, termasuk loading environment variables dan setup global config.

### `internal/app/`
Layer HTTP handler dan routing menggunakan Gin framework.
- `router.go`: Setup routing dan middleware
- `*_handler.go`: HTTP handlers untuk setiap endpoint

### `internal/service/`
Business logic layer (use case layer). Berisi logika bisnis aplikasi.

### `internal/repository/`
Data access layer. Interface dan implementasi untuk akses database (GORM atau raw SQL).

### `internal/model/`
Struct model untuk database. Definisi struct yang digunakan untuk mapping database.

### `internal/websocket/`
WebSocket implementation untuk real-time communication.
- `hub.go`: WebSocket hub untuk manage connections
- `client.go`: WebSocket client implementation
- `ws_handler.go`: WebSocket handler

### `internal/util/`
Utility functions dan helpers:
- `jwt.go`: JWT token generation dan validation
- `hash.go`: Password hashing utilities
- `response.go`: Standard response formatter

### `pkg/logger/`
Reusable logger library yang bisa digunakan di seluruh aplikasi.

## Setup

### Prerequisites
- Go 1.21+
- Docker & Docker Compose
- PostgreSQL
- Redis (optional)
- RabbitMQ (optional)

### Installation

1. Clone repository
```bash
git clone <repository-url>
cd /go
```

2. Copy environment file
```bash
cp .env.example .env
```

3. Update `.env` dengan konfigurasi yang sesuai

4. Install dependencies
```bash
go mod download
```

5. Run dengan Docker Compose
```bash
docker-compose up -d
```

6. Atau run secara lokal
```bash
go run cmd/server/main.go
```

## Environment Variables

Buat file `.env` dengan variabel berikut:

```env
# Server
PORT=5000
SERVER_HOST=0.0.0.0
CLIENT_URL=http://localhost:3000

# Database
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
POSTGRES_USER=your_user
POSTGRES_PASSWORD=your_password
POSTGRES_DB=your_database
POSTGRES_SSLMODE=disable

# JWT
JWT_SECRET=your_jwt_secret_key

# Redis
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=

# RabbitMQ
RABBITMQ_HOST=localhost
RABBITMQ_PORT=5672
RABBITMQ_USER=your_user
RABBITMQ_PASSWORD=your_password
```

## Development

### Run Development Server
```bash
go run cmd/server/main.go
```

### Build
```bash
go build -o bin/server cmd/server/main.go
```

### Run Tests
```bash
go test ./...
```

## Docker

### Build Image
```bash
docker build -t 
```

### Run with Docker Compose
```bash
docker-compose up -d
```

### Stop Services
```bash
docker-compose down
```

## Services & Ports

Setelah menjalankan `docker-compose up -d`, services berikut akan tersedia:

- **API Server**: http://localhost:5000
- **PostgreSQL**: localhost:5432
- **Redis**: localhost:6379
- **RabbitMQ Management UI**: http://localhost:15672
  - Username: `yourapp` (default)
  - Password: `password123` (default)
- **pgweb (Database UI)**: http://localhost:8081
  - Web-based PostgreSQL client untuk melihat dan mengelola database
  - Otomatis terhubung ke database yang dikonfigurasi

## Chat (Real-time 1:1 Messaging)

Chat antar user menggunakan REST API untuk mengirim/mengambil pesan dan WebSocket untuk menerima pesan secara real-time. Hanya user yang sudah berteman (friends) yang dapat saling mengirim pesan.

### Tabel `chat_messages`

| Kolom       | Tipe    | Keterangan              |
|-------------|---------|-------------------------|
| id          | uuid    | PK                      |
| sender_id   | uuid    | FK ke users             |
| receiver_id | uuid    | FK ke users             |
| content     | text    | Isi pesan               |
| is_read     | boolean | Status dibaca           |
| created_at  | timestamp | Waktu dikirim        |
| deleted_at  | timestamp | Soft delete           |

### REST API Endpoints

| Method | Endpoint                           | Keterangan                                           |
|--------|------------------------------------|------------------------------------------------------|
| POST   | `/api/v1/chat/messages`            | Kirim pesan. Body: `{ receiver_id, content }`        |
| GET    | `/api/v1/chat/messages?with_user_id=X&limit=50&offset=0` | Ambil percakapan dengan user X        |
| PUT    | `/api/v1/chat/read/:senderID`      | Tandai pesan dari user X sebagai sudah dibaca        |
| GET    | `/api/v1/chat/unread/count`        | Jumlah pesan belum dibaca                            |

### WebSocket Event

Ketika ada pesan baru dikirim ke user, WebSocket akan mengirim event dengan struktur:

- **Top-level (dari hub)**: `{ type: "notification", payload: { ... } }`
- **Chat message di dalam payload**: `{ type: "chat_message", payload: { id, sender_id, receiver_id, content, created_at, sender } }`

Client perlu memeriksa `data.type === "notification"` dan `data.payload?.type === "chat_message"`, lalu gunakan `data.payload.payload` sebagai objek pesan.

### Alur Frontend

1. User membuka feed, melihat daftar Kontak di sidebar kanan.
2. Klik **Kontak** atau **Friends count** → muncul dialog daftar teman.
3. Klik salah satu teman → muncul `ChatDialog` untuk obrolan 1:1.
4. Kirim pesan via REST; terima pesan baru via WebSocket (`chat_message`).

## Account & Settings

### Delete Account

| Method | Endpoint               | Keterangan                                                                 |
|--------|------------------------|-----------------------------------------------------------------------------|
| DELETE | `/api/v1/auth/account` | Hapus akun (protected). Body: `{ "password": "..." }` untuk login credential. |

- User dengan `login_type: "credential"` wajib menyertakan password.
- User dengan `login_type: "google"` tidak perlu password.
- Soft delete: user dihapus dengan `deleted_at` (GORM).

## Architecture

Aplikasi ini menggunakan **Clean Architecture** dengan layer separation:

1. **Handler Layer** (`internal/app/`): HTTP handlers dan routing
2. **Service Layer** (`internal/service/`): Business logic
3. **Repository Layer** (`internal/repository/`): Data access
4. **Model Layer** (`internal/model/`): Domain models

## License

MIT


Table users {
  id               uuid        [pk, default: `gen_random_uuid()`]
  email            varchar(255) [not null, unique]
  username         varchar(100) [unique]
  phone            varchar(20)
  full_name        varchar(255) [not null]
  password_hash    varchar(255)
  user_type        varchar(50)  [default: 'member']
  profile_photo    text
  date_of_birth    date
  gender           varchar(20)
  is_active        boolean      [default: true]
  is_verified      boolean      [default: false]
  last_login       timestamp
  login_type       varchar(50)  [default: 'credential']
  google_id        varchar(255) [unique]
  otp_code         varchar(6)
  otp_expires_at   timestamp
  reset_token      text
  reset_expires_at timestamp
  created_at       timestamp    [default: `now()`]
  updated_at       timestamp    [default: `now()`]
  deleted_at       timestamp
}

Table profiles {
  id                uuid      [pk, default: `gen_random_uuid()`]
  user_id           uuid      [not null, unique, ref: > users.id]
  bio               text
  cover_photo       text
  website           varchar(500)
  location          varchar(255)
  city              varchar(255)
  country           varchar(255)
  hometown          varchar(255)
  education         varchar(500)
  work              varchar(500)
  relationship_status varchar(50)
  intro             text
  is_profile_public boolean   [default: true]
  created_at        timestamp [default: `now()`]
  updated_at        timestamp [default: `now()`]
}

Table friendships {
  id          uuid      [pk, default: `gen_random_uuid()`]
  sender_id   uuid      [not null, ref: > users.id]
  receiver_id uuid      [not null, ref: > users.id]
  status      varchar(20) [default: 'pending']
  created_at  timestamp [default: `now()`]
  updated_at  timestamp [default: `now()`]

  indexes {
    (sender_id, receiver_id) [unique]
  }
}

Table groups {
  id                uuid      [pk, default: `gen_random_uuid()`]
  created_by        uuid      [not null, ref: > users.id]
  name              varchar(255) [not null]
  description       text
  cover_photo       text
  icon              text
  privacy           varchar(20) [default: 'open']
  membership_policy varchar(30) [default: 'anyone_can_join']
  is_active         boolean   [default: true]
  created_at        timestamp [default: `now()`]
  updated_at        timestamp [default: `now()`]
  deleted_at        timestamp
}

Table posts {
  id             uuid      [pk, default: `gen_random_uuid()`]
  user_id        uuid      [not null, ref: > users.id]
  group_id       uuid      [ref: > groups.id]
  content        text
  post_type      varchar(50) [default: 'text']
  media_url      text
  link_url       text
  shared_post_id uuid      [ref: > posts.id]
  privacy        varchar(20) [default: 'public']
  is_pinned      boolean   [default: false]
  created_at     timestamp [default: `now()`]
  updated_at     timestamp [default: `now()`]
  deleted_at     timestamp
}

Table post_tags {
  id             uuid      [pk, default: `gen_random_uuid()`]
  post_id        uuid      [not null, ref: > posts.id]
  tagged_user_id uuid      [not null, ref: > users.id]
  created_at     timestamp [default: `now()`]

  indexes {
    (post_id, tagged_user_id) [unique]
  }
}

Table post_locations {
  id         uuid      [pk, default: `gen_random_uuid()`]
  post_id    uuid      [not null, unique, ref: > posts.id]
  place_name varchar(255)
  latitude   float
  longitude  float
  created_at timestamp [default: `now()`]
}

Table comments {
  id         uuid      [pk, default: `gen_random_uuid()`]
  post_id    uuid      [not null, ref: > posts.id]
  user_id    uuid      [not null, ref: > users.id]
  parent_id  uuid      [ref: > comments.id]
  content    text      [not null]
  media_url  text
  created_at timestamp [default: `now()`]
  updated_at timestamp [default: `now()`]
  deleted_at timestamp
}

Table likes {
  id          uuid      [pk, default: `gen_random_uuid()`]
  user_id     uuid      [not null, ref: > users.id]
  target_type varchar(20)
  target_id   uuid
  reaction    varchar(20) [default: 'like']
  created_at  timestamp [default: `now()`]

  indexes {
    (user_id, target_type, target_id) [unique]
  }
}

Table chat_messages {
  id          uuid      [pk, default: `gen_random_uuid()`]
  sender_id   uuid      [not null, ref: > users.id]
  receiver_id uuid      [not null, ref: > users.id]
  content     text      [not null]
  is_read     boolean   [default: false]
  created_at  timestamp [default: `now()`]
  deleted_at  timestamp

  indexes {
    (sender_id, receiver_id)
    (receiver_id, sender_id)
  }
}

Table events {
  id          uuid      [pk, default: `gen_random_uuid()`]
  created_by  uuid      [not null, ref: > users.id]
  group_id    uuid      [ref: > groups.id]
  post_id     uuid      [ref: > posts.id]
  title       varchar(255) [not null]
  description text
  cover_photo text
  start_time  timestamp [not null]
  end_time    timestamp
  location    varchar(500)
  privacy     varchar(20) [default: 'public']
  created_at  timestamp [default: `now()`]
  updated_at  timestamp [default: `now()`]
}


flowchart TD
    A([🌐 Buka Facebook]) --> B{Sudah Punya Akun?}

    %% ============ REGISTRASI ============
    B -->|Tidak| C[📋 Klik 'Create New Account']
    C --> D[Isi Nama Lengkap]
    D --> E[Isi Email]
    E --> F[Buat Password]
    F --> G[Isi Tanggal Lahir]
    G --> H[Pilih Gender]
    H --> I[Klik 'Sign Up']
    I --> J[Kirim OTP ke Email]
    J --> JA[Masukkan Kode OTP]
    JA --> JB{OTP Valid?}
    JB -->|Tidak| JC{Coba Ulang?}
    JC -->|Ya| J
    JC -->|Tidak| A
    JB -->|Ya| L[✅ Akun Berhasil Dibuat\nis_verified = true]
    L --> M[Lengkapi Profil - Foto, Bio, dll]
    M --> N[✨ Masuk ke News Feed]

    %% ============ LOGIN ============
    B -->|Ya| O[🔐 Masuk ke Halaman Login]
    O --> P{Pilih Metode Login}
    P -->|Email & Password| Q[Isi Email & Password]
    P -->|Login dengan Google| R[Autentikasi Google OAuth\nSimpan google_id]
    Q --> T{Kredensial Valid?}
    R --> T
    T -->|Tidak| U[❌ Tampilkan Error]
    U --> V{Reset Password?}
    V -->|Ya| W[Kirim Link Reset Token ke Email\nSimpan reset_token + reset_expires_at]
    W --> X[Buka Link & Buat Password Baru]
    X --> Q
    V -->|Tidak| O
    T -->|Ya| Y{is_verified = true?}
    Y -->|Tidak| YA[Kirim Ulang OTP ke Email]
    YA --> YB[Masukkan Kode OTP]
    YB --> YC{OTP Valid?}
    YC -->|Tidak| YA
    YC -->|Ya| YD[Update is_verified = true]
    YD --> N
    Y -->|Ya| N

    %% ============ NEWS FEED & POSTING ============
    N[📰 News Feed] --> AC{Pilih Aktivitas}
    AC -->|Buat Postingan| AD[📝 Klik 'What's on your mind?']
    AC -->|Buat Grup| AX[🏘️ Langsung ke Alur Buat Grup]
    AC -->|Jelajahi / Scroll| AY[🔄 Scroll Feed & Interaksi]
    AD --> AE{Pilih Jenis Postingan}
    AE -->|Teks Saja| AF[Tulis Konten Postingan\npost_type = text]
    AE -->|Foto / Video| AG[Upload Foto / Video\npost_type = photo / video]
    AG --> AF
    AE -->|Link / Artikel| AH[Paste URL Artikel\npost_type = link]
    AH --> AF
    AE -->|Event| AI[Isi Detail Event\npost_type = event]
    AI --> AF
    AF --> AJ{Tag Orang / Lokasi?}
    AJ -->|Ya| AK[Tambahkan Tag → post_tags\nTambahkan Lokasi → post_locations]
    AK --> AL[Pilih Privasi - Public / Friends / Only Me]
    AJ -->|Tidak| AL
    AL --> AM[🔵 Klik 'Post']
    AM --> AN{Post Berhasil?}
    AN -->|Tidak| AO[⚠️ Tampilkan Error - Coba Lagi]
    AO --> AF
    AN -->|Ya| AP[✅ Postingan Tampil di Feed\nSimpan ke posts]
    AP --> AQ{Interaksi di Postingan?}
    AQ -->|Like / Reaksi| AR[👍 Simpan ke likes\ntarget_type = post]
    AQ -->|Komentar| AS[💬 Simpan ke comments\nparent_id = null]
    AQ -->|Reply Komentar| ASR[💬 Reply ke Komentar\nparent_id = comments.id]
    AQ -->|Share| AT[🔁 Buat post baru\npost_type = shared_post]
    AR --> N
    AS --> N
    ASR --> N
    AT --> N

    %% ============ BUAT GRUP ============
    AX --> AU[🏘️ Klik 'Create Group']
    AU --> AV[Isi Nama Grup]
    AV --> AW[Pilih Privasi Grup\nopen / closed / secret]
    AW --> BA[Upload Foto Cover Grup]
    BA --> BB[Tulis Deskripsi Grup]
    BB --> BC[Tambahkan Anggota Awal\n→ group_members role = member]
    BC --> BD[Atur Aturan Grup\n→ Simpan ke group_rules]
    BD --> BE[🔵 Klik 'Create']
    BE --> BF{Grup Berhasil Dibuat?}
    BF -->|Tidak| BG[⚠️ Tampilkan Error]
    BG --> AU
    BF -->|Ya| BH[✅ Grup Berhasil Dibuat!\nCreator → role = admin]
    BH --> BI[Kirim Notifikasi Undangan\n→ notifications type = group_invite]
    BI --> BJ[🏠 Masuk ke Halaman Grup]
    BJ --> BK{Aktivitas di Grup}
    BK -->|Buat Post di Grup| BL[📝 Buat Post\ngroup_id = groups.id]
    BK -->|Kelola Anggota| BM[👥 Approve / Ban Member\n→ Update group_members status]
    BK -->|Edit Grup| BN[⚙️ Edit Pengaturan Grup\n→ Update groups]
    BL --> N
    BM --> N
    BN --> N

    %% ============ STYLING ============
    style A fill:#1877F2,color:#fff,stroke:#none
    style N fill:#1877F2,color:#fff,stroke:#none
    style L fill:#42b983,color:#fff,stroke:#none
    style AP fill:#42b983,color:#fff,stroke:#none
    style BH fill:#42b983,color:#fff,stroke:#none
    style U fill:#e74c3c,color:#fff,stroke:#none
    style AO fill:#e74c3c,color:#fff,stroke:#none
    style BG fill:#e74c3c,color:#fff,stroke:#none