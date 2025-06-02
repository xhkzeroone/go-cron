package jobs

type SayHelloJob struct{}

func (m *SayHelloJob) CronExpr() string {
	//return "*/30 * * * * *"
	return "cron.Every30SecSayHello"
}

func (m *SayHelloJob) Run() {
	println("Every 30 sec task")
}

// Every30SecSayHello
// @Cron cron.Every30SecSayHello
func (m *SayHelloJob) Every30SecSayHello() {
	println("Every 30 sec Every30SecSayHello")
}
