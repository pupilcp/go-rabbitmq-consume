package service

import (
	"context"
	"fmt"
	"github.com/go-resty/resty/v2"
	"github.com/pupilcp/go-rabbitmq-consume/global"
	"github.com/pupilcp/go-rabbitmq-consume/lib/rabbitmq"
	"github.com/pupilcp/go-rabbitmq-consume/model"
	"math"
	"sync"
)

var TaskWorkerMaps = sync.Map{}

type mqWorkerService struct {
}

type TaskWorker struct {
	Task   *model.MqTask
	Amqp   *rabbitmq.AmqpConn
	Ctx    context.Context
	Cancel context.CancelFunc
}

var MqWorkerService mqWorkerService

func (s *mqWorkerService) CreateWorker() {

	taskList := MqTaskService.GetTaskList(1)
	taskLen := len(taskList)
	if taskList == nil || taskLen == 0 {
		return
	}
	//初始化MQ连接池
	rabbitmq.SetMaxConnNum(int(math.Ceil(float64(taskLen) / float64(10))))
	rabbitmq.InitConnPool()
	for _, task := range taskList {
		//每个任务创建一个mq连接
		go s.CreateTaskWorker(task)
	}
}

func (s *mqWorkerService) CreateTaskWorker(task model.MqTask) {
	ctx, cancel := context.WithCancel(context.TODO())
	amqpConn, err := rabbitmq.NewAmqpConn(task.ExchangeName, task.QueueName, task.RouteKey, "direct", true)
	if err != nil {
		global.ServerLogger.Errorf("创建task worker出现错误：%+v", err)
		cancel()
		return
	}
	taskWorker := TaskWorker{
		Task:   &task,
		Amqp:   amqpConn,
		Ctx:    ctx,
		Cancel: cancel,
	}
	s.createTaskWorkerGoroutine(&taskWorker)
	TaskWorkerMaps.Store(task.Id, taskWorker)
	global.ServerLogger.Infof("Task id: %d worker启动成功", task.Id)
}

func (s *mqWorkerService) createTaskWorkerGoroutine(taskWorker *TaskWorker) {
	//根据协程的数量创建消费者
	for num := 1; num <= taskWorker.Task.ConsumerNum; num++ {
		go s.consume(taskWorker)
	}
}

func (s *mqWorkerService) consume(taskWorker *TaskWorker) {
	delivery, err := taskWorker.Amqp.Consume()
	if err != nil {
		global.ServerLogger.Errorf("创建task worker消费consume出现错误：%+v", err)
		return
	}
	cancelChan := taskWorker.Amqp.NoticeCancel()
	closeChan := taskWorker.Amqp.NoticeClose()
	for {
		select {
		case <-taskWorker.Ctx.Done():
			errMsg := fmt.Sprintf("> Task id: %d, worker收到取消信号：%+v", taskWorker.Task.Id, taskWorker.Ctx.Err())
			global.ServerLogger.Error(errMsg)
			AppService.PushNoticeToChannel("MQ消费worker关闭提示", errMsg)
			//关闭channel
			err = taskWorker.Amqp.ChannelClose()
			if err != nil {
				global.ServerLogger.Errorf("task worker关闭channel失败：%+v", err)
			}
			return
		case <-cancelChan:
		case <-closeChan:
			errMsg := fmt.Sprintf("> Task id: %d, worker关闭channel", taskWorker.Task.Id)
			AppService.PushNoticeToChannel("MQ Channel关闭提示", errMsg)
			return
		case d := <-delivery:
			data := string(d.Body)
			if data == "" {
				continue
			}
			global.ServerLogger.Infof("taskWorker收到MQ: %s， 数据：%s", taskWorker.Task.QueueName, data)
			client := resty.New()
			resp, err := client.R().
				//EnableTrace().
				SetHeader("Content-Type", "application/json").
				SetBody(data).
				Post(taskWorker.Task.PushUrl)
			global.ServerLogger.Infof("队列: %s, taskWorker推送: %s， 结果：%+v", taskWorker.Task.QueueName, taskWorker.Task.PushUrl, resp)
			if err != nil {
				global.ServerLogger.Errorf("\"队列: %s, task worker推送：%s出现异常，推送数据：%s,错误信息：%+v", taskWorker.Task.QueueName, taskWorker.Task.PushUrl, data, err)
			}
			_ = d.Ack(false)
		}
	}
}

func (s *mqWorkerService) MonitorTaskConfig() {
	taskList := MqTaskService.GetTaskList(-1)
	if len(taskList) == 0 {
		return
	}
	validTaskLen := 0
	for _, task := range taskList {
		if task.TaskStatus == 1 {
			validTaskLen += 1
		}
	}
	rabbitmq.SetMaxConnNum(int(math.Ceil(float64(validTaskLen) / float64(10))))
	for _, task := range taskList {
		if 1 == task.TaskStatus {
			//任务开启，判断当前是否有存在，不存在则启动
			runningTaskWorker, ok := TaskWorkerMaps.Load(task.Id)
			if !ok {
				s.CreateTaskWorker(task)
			}
			//存在则对比任务的配置是否一致
			if ok {
				runningTask := runningTaskWorker.(TaskWorker).Task
				if runningTask.QueueName != task.QueueName ||
					runningTask.Vhost != task.Vhost ||
					runningTask.ExchangeName != task.ExchangeName ||
					runningTask.RouteKey != task.RouteKey ||
					runningTask.PushUrl != task.PushUrl ||
					runningTask.ConsumerNum != task.ConsumerNum ||
					runningTask.MsgWarningQty != task.MsgWarningQty {
					// 不一致先关闭，再启动
					runningTaskWorker.(TaskWorker).Cancel()
					TaskWorkerMaps.Delete(task.Id)
					s.CreateTaskWorker(task)
					//记录日志并提示
					msg := fmt.Sprintf("系统检测到Task id: %d 配置已更改，关闭该任务的所有worker，并重新启动新的worker", task.Id)
					global.ServerLogger.Info(msg)
					AppService.PushNoticeToChannel("MQ消费worker重启提示", msg)
				}
			}
		}
		if 0 == task.TaskStatus {
			//关闭，如果当前存在该任务，关闭任务
			if runningTaskWorker, ok := TaskWorkerMaps.Load(task.Id); ok {
				runningTaskWorker.(TaskWorker).Cancel()
				TaskWorkerMaps.Delete(task.Id)
				//记录日志并提示
				msg := fmt.Sprintf("系统检测到Task id: %d 状态更改为已停止，已关闭该任务的所有worker", task.Id)
				global.ServerLogger.Info(msg)
				AppService.PushNoticeToChannel("MQ消费worker关闭提示", msg)
			}
		}
	}
}
