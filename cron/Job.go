package cron

type Job interface {
	CronExpr() string
	Run()
}
