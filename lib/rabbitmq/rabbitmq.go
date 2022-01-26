package rabbitmq

import (
	"fmt"
	"github.com/pupilcp/go-rabbitmq-consume/global"
	"github.com/streadway/amqp"
	"math/rand"
	"sync"
	"time"
)

type AmqpConn struct {
	conn         *amqp.Connection
	channel      *amqp.Channel
	vhost        string
	exchangeName string
	routeKey     string
	queueName    string
	kind         string
	isDurable    bool
	delivery     amqp.Delivery
}

type MqConfig struct {
	User  string
	Pwd   string
	Host  string
	Port  string
	Vhost string
}

type ConnectionPool struct {
	Pools   []*amqp.Connection
	ConnNum int
}

var MQConnPool = ConnectionPool{[]*amqp.Connection{}, 0}
var maxConnNum = 1

var wlock sync.RWMutex

func SetMaxConnNum(num int) {
	maxConnNum = num
}

func InitConnPool() {
	for i := 1; i <= maxConnNum; i++ {
		conn, err := Connect()
		if err != nil {
			continue
		}
		MQConnPool.Pools = append(MQConnPool.Pools, conn)
		MQConnPool.ConnNum += 1
	}
}

func GetConnection() (*amqp.Connection, error) {
	if MQConnPool.ConnNum >= maxConnNum && MQConnPool.ConnNum > 0 {
		rand.Seed(time.Now().Unix())
		index := rand.Intn(maxConnNum)
		conn := MQConnPool.Pools[index]
		if !conn.IsClosed() {
			return conn, nil
		}
		wlock.RLock()
		MQConnPool.Pools = append(MQConnPool.Pools[:index], MQConnPool.Pools[index+1:]...)
		MQConnPool.ConnNum -= 1
		wlock.RUnlock()
	}
	connection, err := Connect()
	if err != nil {
		return nil, err
	}
	wlock.RLock()
	MQConnPool.Pools = append(MQConnPool.Pools, connection)
	MQConnPool.ConnNum = MQConnPool.ConnNum + 1
	wlock.RUnlock()
	return connection, nil
}

func Connect() (*amqp.Connection, error) {

	config := MqConfig{
		User:  global.Config.Get("rabbitmq.user").(string),
		Pwd:   global.Config.Get("rabbitmq.password").(string),
		Host:  global.Config.Get("rabbitmq.host").(string),
		Port:  global.Config.Get("rabbitmq.port").(string),
		Vhost: global.Config.Get("rabbitmq.vhost").(string),
	}
	mqUrl := fmt.Sprintf("amqp://%s:%s@%s:%s/%s", config.User, config.Pwd, config.Host, config.Port, config.Vhost)

	connection, err := amqp.Dial(mqUrl)
	if err != nil {
		return nil, err
	}
	return connection, nil
}

func NewAmqpConn(exchangeName, queueName, routeKey, kind string,
	isDurable bool) (*AmqpConn, error) {
	connection, err := GetConnection()
	if err != nil {
		return nil, err
	}
	channel, err := connection.Channel()
	channel.Qos(
		//每次队列只消费一个消息 这个消息处理不完服务器不会发送第二个消息过来
		//当前消费者一次能接受的最大消息数量
		10,
		//服务器传递的最大容量
		0,
		//如果为true 对channel可用 false则只对当前队列可用
		false,
	)
	if err != nil {
		return nil, err
	}
	return &AmqpConn{
		conn:         connection,
		channel:      channel,
		exchangeName: exchangeName,
		queueName:    queueName,
		routeKey:     routeKey,
		kind:         kind,
		isDurable:    isDurable,
	}, nil
}

func (ap *AmqpConn) GetConnection() *amqp.Connection {

	return ap.conn
}

func (ap *AmqpConn) DeclareExchange() (err error) {

	err = ap.channel.ExchangeDeclare(ap.exchangeName, ap.kind, ap.isDurable, false, false, false, nil)
	return
}

func (ap *AmqpConn) DeleteExchange() (err error) {

	err = ap.channel.ExchangeDelete(ap.exchangeName, false, false)
	return
}

func (ap *AmqpConn) DeclareQueue() (err error) {

	_, err = ap.channel.QueueDeclare(ap.queueName, ap.isDurable, false, false, false, nil)
	return
}

func (ap *AmqpConn) BindQueue() (err error) {

	err = ap.channel.QueueBind(ap.queueName, ap.routeKey, ap.exchangeName, false, nil)
	return
}

func (ap *AmqpConn) UnBindQueue() (err error) {

	err = ap.channel.QueueUnbind(ap.queueName, ap.routeKey, ap.exchangeName, nil)
	return
}

func (ap *AmqpConn) deleteQueue() (err error) {

	_, err = ap.channel.QueueDelete(ap.queueName, false, false, false)
	return
}

func (ap *AmqpConn) GetMsgCount(queue string) (int, int, error) {

	q, err := ap.channel.QueueDeclare(queue, true, false, false, false, nil)
	if err != nil {
		return 0, 0, err
	}
	return q.Messages, q.Consumers, nil
}

func (ap *AmqpConn) Publish(data string) (err error) {

	err = ap.channel.Publish(ap.exchangeName, ap.routeKey, false, false, amqp.Publishing{
		ContentType: "text/plain",
		Body:        []byte(data),
	})
	return
}

func (ap *AmqpConn) Consume() (delivery <-chan amqp.Delivery, err error) {

	delivery, err = ap.channel.Consume(
		ap.queueName,
		"",
		false,
		false,
		false,
		false,
		nil)
	if err != nil {
		return nil, err
	}
	return
}

//queue被删除的情况
func (ap *AmqpConn) NoticeCancel() <-chan string {

	return ap.channel.NotifyCancel(make(chan string))
}

//服务器宕机
func (ap *AmqpConn) NoticeClose() <-chan *amqp.Error {

	return ap.channel.NotifyClose(make(chan *amqp.Error))
}

//服务器主动关闭
func (ap *AmqpConn) ConnNoticeClose() <-chan *amqp.Error {

	return ap.conn.NotifyClose(make(chan *amqp.Error))
}

func (ap *AmqpConn) ConnNoticeBloked() <-chan amqp.Blocking {

	return ap.conn.NotifyBlocked(make(chan amqp.Blocking))
}

func (ap *AmqpConn) ConnClose() (err error) {
	if ap.conn.IsClosed() {
		return nil
	}
	err = ap.conn.Close()
	return
}

func (ap *AmqpConn) ChannelClose() (err error) {
	if ap.conn.IsClosed() {
		return nil
	}
	err = ap.channel.Close()
	return
}

//关闭consumer消费通道
func (ap *AmqpConn) DeliveryCancel() (err error) {

	err = ap.channel.Cancel(ap.delivery.ConsumerTag, true)
	return
}
