package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-resty/resty/v2"
	"github.com/pupilcp/go-rabbitmq-consume/global"
	"github.com/pupilcp/go-rabbitmq-consume/lib/rabbitmq"
	"github.com/streadway/amqp"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type NoticeMsg struct {
	Title   string
	Content string
}

type appService struct {
}

var (
	NoticeChannel            = make(chan NoticeMsg, 10)
	mutex                    sync.Mutex
	exitChan                 = make(chan os.Signal)
	serverStart              = time.Now().Format("2006-01-02 15:04:05")
	mqMsgWarningCountMapping = make(map[string]int)
)

var AppService appService

func (s *appService) Run(oper string) {
	switch oper {
	case "start":
		s.start()
	case "stop":
		s.stop()
	case "status":
		s.status()
	case "help":
		s.help()
	}

}

func (s *appService) start() {

	//检查是否已经启动过
	s.checkIsStarted()
	//启动应用，创建MQ消费者
	go MqWorkerService.CreateWorker()

	//监控应用的健康状态
	go s.checkHealth()

	//监控配置，并实现自动重启
	go s.checkTaskConfig()

	//检测MQ队列的挤压数量
	go s.checkMQMsgCount()

	//检测MQ队列消费者数量
	go s.checkMQConsumerCount()

	//发送异常消息处理
	go s.handleNotice()

	go s.setServerPid()

	s.listen()
}

func (s *appService) stop() {
	pid := s.getServerPid()
	if pid == 0 {
		return
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	process.Signal(syscall.SIGTERM)
}

func (s *appService) status() {
	pid := s.getServerPid()
	if pid == 0 {
		fmt.Println(`系统状态：close`)
		return
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		fmt.Println(`系统状态Error：`, err)
		return
	}
	process.Signal(syscall.SIGUSR1)
	time.Sleep(time.Second)
	statusFile := global.Config.GetString("log.logPath") + "/server.status"
	b, err := ioutil.ReadFile(statusFile)
	if err != nil {
		return
	}
	fmt.Println(string(b))
}

func (s *appService) help() {

}

func (s *appService) checkHealth() {
	tick := time.NewTicker(30 * time.Second)
	for {
		select {
		case <-tick.C:
			s.checkMQStatus()
			s.checkDBStatus()
		}
	}
}

func (s *appService) checkMQMsgCount() {
	queueMsgCountCheckInterval := time.Duration(global.Config.GetInt("system.queueMsgCountCheckInterval"))
	if queueMsgCountCheckInterval == 0 {
		return
	}
	tick := time.NewTicker(queueMsgCountCheckInterval * time.Second)
	for {
		select {
		case <-tick.C:
			s.scanQueueMsgCount()
		}
	}
}

func (s *appService) checkMQConsumerCount() {
	queueConsumerCountCheckInterval := time.Duration(global.Config.GetInt("system.queueConsumerCountCheckInterval"))
	if queueConsumerCountCheckInterval == 0 {
		return
	}
	tick := time.NewTicker(queueConsumerCountCheckInterval * time.Second)
	for {
		select {
		case <-tick.C:
			s.scanQueueConsumerCount()
		}
	}
}

func (s *appService) scanQueueConsumerCount() {
	TaskWorkerMaps.Range(func(key, worker interface{}) bool {
		taskWorker := worker.(TaskWorker)
		go func(taskWorker TaskWorker) {
			queueName := taskWorker.Task.QueueName
			_, consumerCount, err := taskWorker.Amqp.GetMsgCount(queueName)
			if err != nil {
				return
			}
			if consumerCount == 0 {
				msg := fmt.Sprintf("> 预警等级：高 \n > 队列名称：%s, 消费者数量：%d", queueName, consumerCount)
				global.ServerLogger.Warning(msg)
				s.PushNoticeToChannel("MQ队列消费者数量提示", msg)
			}
		}(taskWorker)
		return true
	})
}

func (s *appService) scanQueueMsgCount() {
	TaskWorkerMaps.Range(func(key, worker interface{}) bool {
		taskWorker := worker.(TaskWorker)
		go func(taskWorker TaskWorker) {
			queueName := taskWorker.Task.QueueName
			msgCount, consumerCount, err := taskWorker.Amqp.GetMsgCount(queueName)
			if err != nil {
				return
			}
			if msgCount < taskWorker.Task.MsgWarningQty {
				return
			}
			if _, ok := mqMsgWarningCountMapping[queueName]; ok {
				mqMsgWarningCountMapping[queueName] += 1
			} else {
				mqMsgWarningCountMapping[queueName] = 1
			}
			if mqMsgWarningCountMapping[queueName] >= 3 {
				msg := fmt.Sprintf("> 队列名称：%s, 消息数量：%d，当前消费者数量：%d", queueName, msgCount, consumerCount)
				global.ServerLogger.Warning(msg)
				s.PushNoticeToChannel("MQ队列消息数量提示", msg)
				//重置
				mqMsgWarningCountMapping[queueName] = 0
			}
		}(taskWorker)
		return true
	})
}

func (s *appService) checkMQStatus() {
	var conn *amqp.Connection
	var err error
	for i := 1; i <= 3; i++ {
		conn, err = rabbitmq.Connect()
		if err == nil {
			break
		}
		time.Sleep(3 * time.Second)
	}
	if err != nil {
		title := "MQ连接异常提醒"
		content := "系统连续多次检测到MQ连接异常，请留意"
		s.PushNoticeToChannel(title, content)
		global.ServerLogger.Error(content)
	}
	conn.Close()
}

func (s *appService) checkDBStatus() {
	if global.DB.DB().Ping() != nil {
		title := "DB连接关闭提醒"
		content := "系统检测到DB连接关闭，系统尝试重新连接"
		err := global.SetDB()
		if err != nil {
			content += fmt.Sprintf("，连接失败：%+v", err)
		} else {
			content += "，连接成功"
		}
		s.PushNoticeToChannel(title, content)
		global.ServerLogger.Info(content)
	}
}

func (s *appService) checkTaskConfig() {
	configCheckInterval := time.Duration(global.Config.GetInt("system.configCheckInterval"))
	tick := time.NewTicker(configCheckInterval * time.Second)
	for {
		select {
		case <-tick.C:
			//防止并发
			mutex.Lock()
			MqWorkerService.MonitorTaskConfig()
			mutex.Unlock()
		}
	}
}

func (s *appService) PushNoticeToChannel(title, content string) {
	if !global.Config.Get("system.enableWebhook").(bool) {
		return
	}
	NoticeChannel <- NoticeMsg{
		Title:   title,
		Content: content,
	}
}

func (s *appService) handleNotice() {
	for msg := range NoticeChannel {
		_ = s.notify(msg.Title, msg.Content)
	}
}

func (s *appService) notify(title string, content ...string) error {
	data := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]interface{}{
			"content": "#### " + title + "\n" + strings.Join(content, "\n"),
		},
	}
	url := global.Config.Get("system.webhookUrl").(string)
	if url == "" {
		return errors.New("请配置webhookUrl")
	}
	resp, err := resty.New().R().SetBody(data).
		SetHeader("'Content-Type", "application/json").
		Post(url)
	if err != nil {
		global.ServerLogger.Errorf("推送webhook失败，错误信息：%+v", err)
		return err
	}
	global.ServerLogger.Errorf("推送webhook结果：%s", string(resp.Body()))

	result := make(map[string]interface{})
	_ = json.Unmarshal(resp.Body(), result)
	if code, ok := result["errcode"]; ok {
		if code.(int) > 0 {
			return errors.New(result["errmsg"].(string))
		}
		return nil
	} else {
		return errors.New(fmt.Sprintf("未知错误：%+v", result))
	}
}

func (s *appService) setServerPid() {
	//写入主进程ID
	pidPath := global.Config.GetString("system.pidPath")
	_, err := os.Stat(pidPath)
	if err != nil && os.IsNotExist(err) {
		_ = os.Mkdir(pidPath, os.ModePerm)
	}
	pidFile := pidPath + string(filepath.Separator) + "server.pid"
	_, err = os.Stat(pidFile)
	var pidFileInfo *os.File
	if err != nil && os.IsNotExist(err) {
		pidFileInfo, _ = os.Create(pidFile)
	} else {
		pidFileInfo, _ = os.OpenFile(pidFile, os.O_RDWR|os.O_TRUNC, 0666)
	}
	_, _ = pidFileInfo.WriteString(strconv.Itoa(os.Getpid()))
}

func (s *appService) getServerPid() int {
	//获取主进程ID
	pidFile := global.Config.GetString("system.pidPath") + string(filepath.Separator) + "server.pid"
	_, err := os.Stat(pidFile)
	if err != nil {
		return 0
	}
	b, err := ioutil.ReadFile(pidFile)
	if err != nil {
		return 0
	}
	pid, _ := strconv.Atoi(string(b))

	return pid
}

func (s *appService) clearServerPid() {
	//获取主进程ID
	pidFile := global.Config.GetString("system.pidPath") + string(filepath.Separator) + "server.pid"
	_, err := os.Stat(pidFile)
	if err != nil {
		return
	}
	fp, err := os.OpenFile(pidFile, os.O_TRUNC|os.O_RDONLY, 0666)
	defer fp.Close()
}

func (s *appService) listen() {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println(err)
		}
	}()
	signal.Notify(exitChan, syscall.SIGTERM, syscall.SIGUSR1)
	for {
		select {
		case signals := <-exitChan:
			fmt.Println(signals)
			if signals.String() == "user defined signal 1" {
				s.showStatus()
			}
			if signals.String() == "terminated" {
				s.stopSmonthly()
				return
			}
		}
	}
}

func (s *appService) stopSmonthly() {
	msg := fmt.Sprintf("系统收到终止信号，将关闭所有MQWorker")
	fmt.Println(msg)
	global.ServerLogger.Error(msg)
	TaskWorkerMaps.Range(func(key, worker interface{}) bool {
		worker.(TaskWorker).Cancel()
		TaskWorkerMaps.Delete(key)
		return true
	})
	//清空server pid
	s.clearServerPid()
	time.Sleep(3 * time.Second)
	s.PushNoticeToChannel("[go-rabbitmq-consume]系统退出提醒", msg)
	fmt.Println("系统已退出")
}

func (s *appService) showStatus() {
	_, isLive := s.isStarted()
	msg := ""
	if isLive {
		msg = "系统状态：running\n"
		msg += "启动时间：" + serverStart + "\n"
		TaskWorkerMaps.Range(func(key, worker interface{}) bool {
			taskWorker := worker.(TaskWorker)
			msg += fmt.Sprintf("- Task worker id: %d, 队列名：%s 正在运行\n", taskWorker.Task.Id, taskWorker.Task.QueueName)
			isLive = true
			return true
		})
	} else {
		msg = `系统状态：close`
	}
	//写入状态日志
	statusFile := global.Config.GetString("log.logPath") + "/server.status"
	fp, err := os.OpenFile(statusFile, os.O_TRUNC|os.O_RDWR|os.O_CREATE, 0666)
	defer fp.Close()
	if err != nil {
		return
	}
	fp.WriteString(msg)
}

func (s *appService) checkIsStarted() {
	pid, status := s.isStarted()
	if status {
		log.Fatalf("系统正在运行，无需重复启动，进程ID：%d", pid)
	}
}

func (s *appService) isStarted() (int, bool) {
	pid := s.getServerPid()
	if pid == 0 {
		return 0, false
	}
	if err := syscall.Kill(pid, 0); err != nil {
		return 0, false
	}
	return pid, true
}
