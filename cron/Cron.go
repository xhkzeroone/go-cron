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

// Cron là struct thực thi interface Scheduler, dùng để quản lý cron jobs.
type Cron struct {
	*cron.Cron
}

// NewCronJob tạo mới một đối tượng Cron.
//
// Cách sử dụng:
//
//	cron := cron.NewCronJob()
func NewCronJob() *Cron {
	return &Cron{
		Cron: cron.New(cron.WithSeconds()),
	}
}

// AddJob thêm một cron job mới vào cron scheduler.
//
// Parameters:
//   - cronExpr: biểu thức cron (ví dụ: "0 0 * * * *" chạy mỗi giờ)
//   - jobFunc: hàm sẽ được thực thi theo lịch.
//
// Trả về lỗi nếu biểu thức cron không hợp lệ hoặc không thể thêm job.
//
// Cách sử dụng:
//
//	cron.AddJob("0 0 * * * *", myFunc)
func (c *Cron) AddJob(cronExpr string, jobFunc func()) error {
	if !isValidCronExpr(cronExpr) {
		cronExpr = viper.GetString(cronExpr)
		if !isValidCronExpr(cronExpr) {
			return fmt.Errorf("invalid cron expr: %s", cronExpr)
		}
	}
	_, err := c.AddFunc(cronExpr, jobFunc)
	if err != nil {
		return fmt.Errorf("failed to add cron job: %v", err)
	}
	return nil
}

// Register đăng ký các handler struct có chứa các hàm cron cần gọi.
//
// Cách sử dụng:
//
// Register đăng ký các handler struct có chứa các hàm cron cần gọi.
//
// Cách sử dụng:
//
//	cron.Register(MyJobHandler{}, AnotherHandler{})
func (c *Cron) Register(jobs ...Job) {
	if len(jobs) == 0 {
		return
	}

	for _, job := range jobs {
		jobName := reflect.TypeOf(job).String()
		cronExpr := job.CronExpr()

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

		log.Printf("registered cron job %s with expression [%s]", jobName, cronExpr)
	}
}

// RegisterByTags đăng ký các handler struct có chứa các hàm cron cần gọi.
//
// Cách sử dụng:
//
// RegisterByTags đăng ký các handler struct có chứa các hàm cron cần gọi.
//
// Cách sử dụng:
//
//	cron.RegisterByTags(MyJobHandler{}, AnotherHandler{})
func (c *Cron) RegisterByTags(jobs ...interface{}) {
	if len(jobs) == 0 {
		return
	}

	for _, job := range jobs {
		val := reflect.ValueOf(job)
		typ := reflect.TypeOf(job)

		// Lấy file path của struct để quét comment
		filePath, err := getFilePathOfStruct(job)
		if err != nil {
			log.Printf("failed to get file path of struct %T: %v", job, err)
			continue
		}

		// Lấy map methodName -> cronExpr từ comment
		methodCronMap, err := parseCronTags(filePath)
		if err != nil {
			log.Printf("failed to parse cron tags for %T: %v", job, err)
			continue
		}

		for i := 0; i < typ.NumMethod(); i++ {
			method := typ.Method(i)
			if expr, ok := methodCronMap[method.Name]; ok {
				// Đảm bảo method không có tham số
				if method.Type.NumIn() != 1 || method.Type.NumOut() != 0 {
					log.Printf("skip method %s: unsupported signature", method.Name)
					continue
				}

				fn := func() {
					method.Func.Call([]reflect.Value{val})
				}

				if err := c.AddJob(expr, fn); err != nil {
					log.Printf("failed to register method %s: %v", method.Name, err)
				} else {
					log.Printf("registered cron job %s with expr %s", method.Name, expr)
				}
			}
		}
	}
}

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
	file, _ := fn.FileLine(pc)
	return file, nil
}

func parseCronTags(filePath string) (map[string]string, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	result := make(map[string]string)

	for _, decl := range node.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv != nil && fn.Doc != nil {
			for _, comment := range fn.Doc.List {
				if strings.HasPrefix(comment.Text, "// @Cron ") {
					expr := strings.TrimSpace(strings.TrimPrefix(comment.Text, "// @Cron "))
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

// isValidCronExpr kiểm tra tính hợp lệ của biểu thức cron.
//
// Cách sử dụng nội bộ: dùng trong AddJob().
func isValidCronExpr(expr string) bool {
	_, err := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow).Parse(expr)
	return err == nil
}
