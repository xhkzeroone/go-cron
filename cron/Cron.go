// Package cron cung cấp một lớp tiện ích đơn giản để quản lý và chạy các tác vụ định kỳ
// (cron jobs) trong ứng dụng Go. Gói này dựa trên thư viện github.com/robfig/cron/v3
// và bổ sung các tiện ích:
//   - Hỗ trợ biểu thức cron có giây (6 trường thay vì 5).
//   - Cho phép trích xuất biểu thức cron từ file cấu hình (viper).
//   - Đăng ký tác vụ bằng phương thức (implements Job) hoặc bằng comment tag
//     "// @Cron <expr>" ngay trên method.
//   - Xác thực biểu thức cron và log lỗi chi tiết.
//
// Tác giả: <Your Name>
// Ngày tạo: 20‑06‑2025
package cron

import (
	"fmt"
	"github.com/robfig/cron/v3"
	"github.com/spf13/viper"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"reflect"
	"runtime"
	"strings"
)

// ----------------------------------------------------------------------------
// Kiểu dữ liệu & Constructor
// ----------------------------------------------------------------------------

// Cron là wrapper của *cron.Cron, triển khai interface Scheduler của robfig/cron.
// Bạn có thể gọi tất cả các hàm gốc của *cron.Cron trên đối tượng này.
//
// Ví dụ:
//
//	c := cron.NewCronJob()
//	c.Start() // Bắt đầu chạy scheduler ở goroutine riêng.
//	defer c.Stop() // Dừng scheduler khi thoát.
//
// Ngoài ra, gói còn cung cấp các helper AddJob, Register, RegisterByTags
// để đăng ký tác vụ theo nhiều cách thuận tiện hơn.
type Cron struct {
	*cron.Cron // embedding để sử dụng trực tiếp các API gốc
}

// NewCronJob khởi tạo một Scheduler mới có hỗ trợ trường giây trong biểu thức cron.
//
//	c := cron.NewCronJob()
//
// Sau khi tạo, bạn có thể gọi:
//   - c.AddJob(expr, fn)
//   - c.Register(jobImpls...)
//   - c.RegisterByTags(handlerStructs...)
//   - c.Start() – nên được gọi ở goroutine chính của ứng dụng.
func NewCronJob() *Cron {
	return &Cron{
		Cron: cron.New(cron.WithSeconds()),
	}
}

// ----------------------------------------------------------------------------
// Đăng ký tác vụ trực tiếp bằng hàm (func())
// ----------------------------------------------------------------------------

// AddJob thêm một cron job mới vào scheduler.
//
// Tham số:
//   - cronExpr – biểu thức cron 6 trường (vd: "0 0 * * * *" chạy đầu mỗi giờ).
//     Nếu expr không hợp lệ, hàm sẽ thử đọc expr từ file cấu hình
//     thông qua viper.GetString(cronExpr).
//   - jobFunc  – hàm không tham số, không giá trị trả về, sẽ được gọi định kỳ.
//
// Hàm trả về error nếu biểu thức cron không hợp lệ hoặc c.AddFunc() thất bại.
//
// Ví dụ:
//
//	err := c.AddJob("*/5 * * * * *", func(){ fmt.Println("ping") })
//	if err != nil { log.Fatal(err) }
func (c *Cron) AddJob(cronExpr string, jobFunc func()) error {
	// Nếu expr không hợp lệ => thử đọc cấu hình với key = cronExpr
	if !isValidCronExpr(cronExpr) {
		cronExpr = viper.GetString(cronExpr)
		if !isValidCronExpr(cronExpr) {
			return fmt.Errorf("invalid cron expr: %s", cronExpr)
		}
	}

	// Đăng ký job với scheduler nội bộ
	if _, err := c.AddFunc(cronExpr, jobFunc); err != nil {
		return fmt.Errorf("failed to add cron job: %v", err)
	}
	return nil
}

// ----------------------------------------------------------------------------
// Đăng ký tác vụ bằng cách implement interface Job
// ----------------------------------------------------------------------------

// Job là interface tối giản để mô tả một tác vụ cron.
// Bất kỳ struct nào implement hai hàm bên dưới đều có thể truyền vào c.Register().
//   • CronExpr() string – trả về biểu thức cron.
//   • Run()             – logic sẽ được scheduler gọi theo lịch.
//
// Ví dụ:
//  type HourlyReport struct{}
//  func (HourlyReport) CronExpr() string { return "0 0 * * * *" }
//  func (HourlyReport) Run()             { doSomething() }
//
//  c.Register(HourlyReport{})
//
// Lưu ý: Interface này được định nghĩa ở đây để tránh circular import.
// type Job interface {
// 	CronExpr() string
// 	Run()
// }

// Register nhận một danh sách Job, đọc biểu thức cron bằng job.CronExpr(),
// validate và thêm vào scheduler. Hữu ích khi bạn muốn gom mỗi job logic
// thành một struct riêng biệt.
func (c *Cron) Register(jobs ...Job) {
	if len(jobs) == 0 {
		return
	}

	for _, job := range jobs {
		jobName := reflect.TypeOf(job).String()
		cronExpr := job.CronExpr()

		// Cho phép mapping expr thông qua viper key
		if !isValidCronExpr(cronExpr) {
			cronExpr = viper.GetString(cronExpr)
			if !isValidCronExpr(cronExpr) {
				log.Printf("invalid cron expr for job %s: %s", jobName, cronExpr)
				continue
			}
		}

		if err := c.AddJob(cronExpr, job.Run); err != nil {
			log.Printf("failed to add cron job %s: %v", jobName, err)
			continue
		}
		log.Printf("registered cron job %s với biểu thức [%s]", jobName, cronExpr)
	}
}

// ----------------------------------------------------------------------------
// Đăng ký tác vụ thông qua comment tag // @Cron <expr>
// ----------------------------------------------------------------------------

// RegisterByTags quét struct handler và tự động đăng ký các method có comment
// dạng "// @Cron <expr>" ngay phía trên method. Điều này giúp code sạch sẽ,
// không cần implement interface hay gọi thủ công AddJob.
//
// Điều kiện method:
//   - Không có tham số (ngoại trừ receiver) và không trả về giá trị.
//   - Comment phải đúng định dạng // @Cron <biểu_thức_6_trường>.
//   - Biểu thức có thể là key cấu hình viper (tương tự AddJob).
//
// Ví dụ:
//
//	type MyHandler struct{}
//	// @Cron 0 */30 * * * *
//	func (MyHandler) CleanTemp()   { ... }
//	// @Cron backup.cronExpr (key trong file config)
//	func (MyHandler) BackupDB()    { ... }
//
//	c.RegisterByTags(MyHandler{})
func (c *Cron) RegisterByTags(jobs ...interface{}) {
	if len(jobs) == 0 {
		return
	}

	for _, job := range jobs {
		val := reflect.ValueOf(job) // receiver value
		typ := reflect.TypeOf(job)  // receiver type

		// Lấy đường dẫn file chứa struct để phân tích comment
		filePath, err := getFilePathOfStruct(job)
		if err != nil {
			log.Printf("failed to get file path of struct %T: %v", job, err)
			continue
		}

		// Parse comment và lấy map[MethodName]CronExpr
		methodCronMap, err := parseCronTags(filePath)
		if err != nil {
			log.Printf("failed to parse cron tags for %T: %v", job, err)
			continue
		}

		// Duyệt toàn bộ method của struct
		for i := 0; i < typ.NumMethod(); i++ {
			method := typ.Method(i)
			if expr, ok := methodCronMap[method.Name]; ok {
				// Đảm bảo chữ ký method: func (T) Method()
				if method.Type.NumIn() != 1 || method.Type.NumOut() != 0 {
					log.Printf("skip method %s: unsupported signature", method.Name)
					continue
				}

				// Bao đóng method thành func() để AddJob
				fn := func() {
					method.Func.Call([]reflect.Value{val})
				}

				if err := c.AddJob(expr, fn); err != nil {
					log.Printf("failed to register method %s: %v", method.Name, err)
				} else {
					log.Printf("registered cron job %s với biểu thức %s", method.Name, expr)
				}
			}
		}
	}
}

// ----------------------------------------------------------------------------
// Helper: Tìm file path & phân tích comment tag
// ----------------------------------------------------------------------------

// getFilePathOfStruct trả về đường dẫn file .go định nghĩa struct.
// Thực hiện bằng cách lấy con trỏ hàm đầu tiên của struct rồi tra cứu runtime.
func getFilePathOfStruct(i interface{}) (string, error) {
	typ := reflect.TypeOf(i)
	if typ.NumMethod() == 0 {
		return "", fmt.Errorf("struct %T has no methods", i)
	}

	pc := typ.Method(0).Func.Pointer()
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return "", fmt.Errorf("cannot find function for struct")
	}

	file, _ := fn.FileLine(pc) // chỉ cần file, bỏ qua line
	return file, nil
}

// parseCronTags đọc file Go, tìm các method có receiver và comment // @Cron ...
// Trả về map tênMethod -> biểu thức cron.
func parseCronTags(filePath string) (map[string]string, error) {
	fset := token.NewFileSet()

	// nil src => parser tự đọc từ filePath
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	result := make(map[string]string)

	// Duyệt tất cả khai báo (Decls) và lọc FuncDecl có receiver
	for _, decl := range node.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv != nil && fn.Doc != nil {
			// Lấy từng dòng comment của function
			for _, comment := range fn.Doc.List {
				if strings.HasPrefix(comment.Text, "// @Cron ") {
					expr := strings.TrimSpace(strings.TrimPrefix(comment.Text, "// @Cron "))

					// Cho phép expr là key viper
					if !isValidCronExpr(expr) {
						expr = viper.GetString(expr)
						if !isValidCronExpr(expr) {
							log.Printf("invalid cron expr for job %s: %s", fn.Name.Name, expr)
							continue
						}
					}
					result[fn.Name.Name] = expr
				}
			}
		}
	}

	return result, nil
}

// ----------------------------------------------------------------------------
// Validator Helper
// ----------------------------------------------------------------------------

// isValidCronExpr kiểm tra tính hợp lệ của biểu thức cron (6 trường giây‑phút‑giờ‑ngày‑tháng‑thứ).
// Sử dụng parser mặc định của robfig/cron, trả về true nếu parse thành công.
func isValidCronExpr(expr string) bool {
	_, err := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow).Parse(expr)
	return err == nil
}
