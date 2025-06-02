package main

import (
	"github.com/xhkzeroone/go-config/config"
	"github.com/xhkzeroone/go-cron/cron"
	"github.com/xhkzeroone/go-cron/example/jobs"
)

type Config struct {
}

func main() {
	err := config.LoadConfig(&Config{})
	if err != nil {
		return
	}
	c := cron.NewCronJob()
	c.Register(&jobs.SayHelloJob{}) // Đăng ký handler chứa các hàm
	c.RegisterByTags(&jobs.SayHelloJob{})
	c.Start()

	select {} // block mãi mãi
}
