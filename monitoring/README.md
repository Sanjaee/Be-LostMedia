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
| 6      | **prometheus** | **9090** | Metrik CPU, RAM, dll.      |
| 7      | node_exporter | 9100 | Metrik host (CPU, RAM, disk)   |
| 8      | cadvisor  | 8082  | Metrik per container            |
| 9      | grafana   | **3001** | UI monitoring & log          |
| 10     | app       | 5000  | Backend Go                       |

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

## 3. Cara munculkan logging & monitoring

Supaya **log dan monitoring** langsung terlihat (bukan hanya "Welcome to Grafana"):

### A. Cek data source Loki

1. Klik **Connections** (ikon plug) di menu kiri → **Data sources**.
2. Harus ada **Loki** dengan status hijau. Kalau belum ada, klik **Add data source** → pilih **Loki** → URL: `http://loki:3100` → **Save & test**.

### B. Lihat log (Explore)

1. Klik **Explore** (ikon kompas) di menu kiri.
2. Pilih data source **Loki** di dropdown atas.
3. Query: **`{job="yourapp"}`** → **Run query**.
4. Pilih time range **Last 15 minutes** atau **Last 1 hour**.
5. Log backend akan muncul di bawah.

### C. Dashboard monitoring

1. Klik **Dashboards** (ikon kotak) di menu kiri.
2. Buka folder **Monitoring**:
   - **Backend Logs** — log aplikasi (Loki), refresh 30 detik.
   - **System metrics (CPU, RAM)** — CPU & RAM host + per container (Prometheus + Node Exporter + cAdvisor).

Untuk **monitoring CPU, RAM, dll.**:
- Buka **Connections** → **Data sources** → pastikan **Prometheus** ada (URL: `http://prometheus:9090`).
- Buka **Dashboards** → **Monitoring** → **System metrics (CPU, RAM)**.
- Di sana: CPU usage (host), Memory usage (host), CPU/Memory per container Docker.

Kalau dashboard belum muncul, restart Grafana sekali agar provisioning terbaca:

```bash
docker compose restart grafana
```

Lalu buka lagi **Dashboards** → **Monitoring**.

**Catatan Windows:** Kalau **node_exporter** atau **cadvisor** gagal start (error mount `/proc`, `/sys`), di Windows Docker Desktop path host bisa berbeda. Cek log: `docker logs yourapp_node_exporter` dan `docker logs yourapp_cadvisor`. Dashboard **System metrics** tetap bisa dipakai untuk metrik dari cAdvisor (per container) meski Node Exporter tidak jalan.

**Kalau "Container CPU usage" No data:**
1. Cek cAdvisor jalan: `docker compose ps` → **cadvisor** harus **Up**.
2. Cek Prometheus scrape: buka **http://localhost:9090/targets** → job **cadvisor** harus **UP** (hijau).
3. Kalau cadvisor error start (mis. di Windows), baca log: `docker logs yourapp_cadvisor`; sering karena volume mount. Bisa comment dulu volume yang bermasalah di `docker-compose.yml` (service cadvisor).

**Filesystem usage angka aneh (overflow):** Sudah diperbaiki di dashboard (pakai `clamp_max`). Refresh dashboard atau restart Grafana.

**"Live tailing was stopped due to following error: undefined":** Biasanya karena Nginx di depan Grafana tidak support WebSocket. Di `nginx.conf` blok `location /grafana/` harus ada `proxy_set_header Upgrade $http_upgrade` dan `proxy_set_header Connection $connection_upgrade` (plus `map $http_upgrade $connection_upgrade` di atas). Pakai time range kecil (Last 5 minutes) saat Live.

---

## 4. Menu yang muncul dan kegunaannya

### Menu kiri (sidebar)

| Menu        | Ikon        | Kegunaan                                      |
|-------------|-------------|-----------------------------------------------|
| **Home**    | rumah       | Dashboard awal, shortcut                     |
| **Explore** | kompas      | **Tempat lihat log dari Loki** (pakai ini)    |
| **Dashboards** | kotak   | Daftar folder & dashboard (ada **Backend Logs**) |
| **Alerting**   | lonceng | Atur alert (opsional)                        |
| **Connections**| plug   | Data sources (Loki), plugin                   |
| **Administration** | gear | User, org, data sources, plugin |

Loki sudah di-provision sebagai **data source**, jadi tidak perlu tambah manual.

---

## 5. Melihat log (Loki) di Grafana

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

## 6. Cek Loki & Promtail jalan

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

## 7. Kenapa "No data" dan cara pastikan log masuk

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
