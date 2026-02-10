# Setup Grafana + Loki + Promtail (dari awal sampai menu muncul)

Panduan urut dari menjalankan stack sampai log terlihat di Grafana.

---

## 1. Yang dijalankan di awal

Dari folder **`be`** (root backend):

```bash
cd C:\Users\afriz\OneDrive\Desktop\sosialmedia\be
docker compose up -d
```

Ini akan start (dengan urutan dependency):

| Urutan | Service   | Port  | Fungsi                          |
|--------|-----------|-------|----------------------------------|
| 1      | db        | 5432  | PostgreSQL                       |
| 2      | redis     | 6379  | Redis                            |
| 3      | rabbitmq  | 5672, 15672 | RabbitMQ                  |
| 4      | **loki**  | **3100** | Simpan & API log               |
| 5      | promtail  | (internal) | Kirim log ke Loki            |
| 6      | grafana   | **3001** | UI monitoring & log          |
| 7      | app       | 5000  | Backend Go                       |

Cek semua container jalan:

```bash
docker compose ps
```

Semua status harus **Up** (atau **running**).

---

## 2. Buka Grafana

1. Di browser buka: **http://localhost:3001**
2. Login:
   - **Username:** `admin`
   - **Password:** `admin`  
   (atau pakai `GRAFANA_ADMIN_USER` / `GRAFANA_ADMIN_PASSWORD` dari env)
3. Jika diminta ganti password, bisa **Skip** dulu atau ganti.

Setelah login, kamu masuk ke **Home** Grafana.

---

## 3. Menu yang muncul dan kegunaannya

### Menu kiri (sidebar)

| Menu        | Ikon        | Kegunaan                                      |
|-------------|-------------|-----------------------------------------------|
| **Home**    | rumah       | Dashboard awal, shortcut                     |
| **Explore** | kompas      | **Tempat lihat log dari Loki** (pakai ini)    |
| **Dashboards** | kotak   | Daftar folder & dashboard                     |
| **Alerting**   | lonceng | Atur alert (opsional)                        |
| **Connections**| plug   | Data sources, plugin, dll                    |
| **Administration** | gear | User, org, data sources, plugin |

Loki sudah di-provision sebagai **data source**, jadi tidak perlu tambah manual.

---

## 4. Melihat log (Loki) di Grafana

1. Klik **Explore** (ikon kompas) di menu kiri.
2. Di atas, pilih data source: **Loki** (dropdown).
3. Di kotak **Label filter** (atau query), isi salah satu:
   - Semua log yang dikirim ke Loki:  
     `{}`
   - Log dengan label job yourapp:  
     `{job="yourapp"}`
   - Log dari file app (jika pakai Promtail file):  
     `{app="go-backend"}`
4. Pilih **Time range** (kanan atas), misalnya “Last 1 hour”.
5. Klik **Run query** (atau Shift+Enter).

Log akan muncul di bawah dalam bentuk garis waktu (log lines).

---

## 5. Cek Loki & Promtail jalan

- **Loki:**  
  Buka http://localhost:3100/ready  
  Harus dapat respons **ready** (bukan 404 untuk path lain).

- **Promtail:**  
  Log container Promtail:
  ```bash
  docker logs yourapp_promtail
  ```
  Tidak error dan ada koneksi ke Loki = OK.

---

## 6. Kenapa "No data" dan cara pastikan log masuk

- Log app **masuk ke Loki** karena backend Go menulis ke file `/var/log/app/app.log` dan Promtail mengirim file itu ke Loki.
- Di Explore pakai query: **`{job="yourapp"}`** atau **`{app="go-backend"}`**.
- Jika tetap "No data":
  1. Pastikan **app** dan **Promtail** sudah restart setelah perubahan:  
     `docker compose up -d --build app promtail`
  2. Tunggu ~1 menit lalu pilih time range **Last 5 minutes** atau **Last 15 minutes** dan **Run query** lagi.
  3. Cek log Promtail: `docker logs yourapp_promtail` (tidak boleh error).

---

## Ringkasan singkat

1. **Jalankan:** `docker compose up -d` di folder `be`.
2. **Buka Grafana:** http://localhost:3001 → login `admin` / `admin`.
3. **Lihat log:** menu **Explore** → pilih **Loki** → query `{}` atau `{job="yourapp"}` → **Run query**.

Dari sini semua menu Grafana (Home, Explore, Dashboards, Alerting, Connections, Administration) sudah muncul dan siap dipakai; untuk log, yang dipakai adalah **Explore** + data source **Loki**.
