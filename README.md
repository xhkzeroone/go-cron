# Cron – Scheduler Utility for Go

> **TL;DR**: 📟 Dễ dàng định nghĩa, đăng ký và chạy các cron‑job có độ chính xác tới giây trong ứng dụng Go của bạn.

## ✨ Tính năng nổi bật

| Tính năng              | Mô tả                                                                          |
| ---------------------- | ------------------------------------------------------------------------------ |
| **Biểu thức 6 trường** | Hỗ trợ giây (second) – `0 0 * * * *` chạy đầu mỗi giờ.                         |
| **Tích hợp Viper**     | Cho phép đặt biểu thức cron trong file cấu hình và tham chiếu bằng key.        |
| **Đăng ký linh hoạt**  | 3 cách đăng ký job: `AddJob(func)`, `Register(Job)`, `RegisterByTags(struct)`. |
| **Comment Tag**        | Chỉ cần thêm `// @Cron <expr>` ngay trên method – giữ code sạch sẽ.            |
| **Log chi tiết**       | Validate biểu thức và ghi log khi job được đăng ký/thất bại.                   |

## 🔧 Cài đặt

```bash
go get github.com/xhkzeroone/cron@latest
```

> Gói phụ thuộc:
>
> * [`robfig/cron/v3`](https://github.com/robfig/cron)
> * [`spf13/viper`](https://github.com/spf13/viper)

## 🚀 Bắt đầu nhanh

### 1. Khởi tạo Scheduler

```go
c := cron.NewCronJob()
c.Start()

defer c.Stop()
```

### 2. Đăng ký job bằng hàm ẩn danh

```go
c.AddJob("*/5 * * * * *", func() {
    fmt.Println("Ping mỗi 5 giây")
})
```

### 3. Đăng ký job bằng interface `Job`

```go
// Định nghĩa Job
type HourlyReport struct{}
func (HourlyReport) CronExpr() string { return "0 0 * * * *" }
func (HourlyReport) Run()             { generateReport() }

// Đăng ký
c.Register(HourlyReport{})
```

### 4. Đăng ký job bằng comment tag

```go
type CleanupHandler struct{}

// @Cron 0 */30 * * * *  // mỗi 30 phút
func (CleanupHandler) CleanTemp() {
    os.RemoveAll("/tmp/cache")
}

// @Cron backup.cronExpr  // đọc từ file config
func (CleanupHandler) BackupDB() {
    backupDatabase()
}

c.RegisterByTags(CleanupHandler{})
```

## 📝 Cấu hình (Viper)

Giả sử bạn có file `config.yaml`:

```yaml
backup:
  cronExpr: "0 0 3 * * *" # 03:00 mỗi ngày
```

Khi dùng `viper.GetString("backup.cronExpr")`, gói sẽ tự ánh xạ và validate.

## 🖇️ API Reference

### `func NewCronJob() *Cron`

Trả về scheduler mới có hỗ trợ giây.

### `func (c *Cron) AddJob(expr string, fn func()) error`

Thêm job bằng hàm. `expr` có thể là biểu thức hợp lệ hoặc **key** cấu hình Viper.

### `func (c *Cron) Register(jobs ...Job)`

Đăng ký hàng loạt struct implement `Job` (gồm `CronExpr()` & `Run()`).

### `func (c *Cron) RegisterByTags(handlers ...interface{})`

Quét comment `// @Cron` trong struct và tự động đăng ký.

## 🛠️ Phát triển

```bash
# Chạy lint & test before commit
go vet ./...
go test ./...
```

## 📄 License

MIT © 2025 Your Name
