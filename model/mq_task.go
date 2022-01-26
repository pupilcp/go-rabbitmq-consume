package model

type MqTask struct {
	Id            int    `gorm:"column:id;primary_key" json:"id"`
	TaskName      string `gorm:"column:task_name;NOT NULL" json:"task_name"`
	Vhost         string `gorm:"column:vhost;NOT NULL" json:"vhost"`
	ExchangeName  string `gorm:"column:exchange_name;NOT NULL" json:"exchange_name"`
	RouteKey      string `gorm:"column:route_key;NOT NULL" json:"route_key"`
	QueueName     string `gorm:"column:queue_name;NOT NULL" json:"queue_name"`
	PushUrl       string `gorm:"column:push_url;NOT NULL" json:"push_url"`
	ConsumerNum   int    `gorm:"column:consumer_num;default:0;NOT NULL" json:"consumer_num"`
	MsgWarningQty int    `gorm:"column:msg_warning_qty;default:0;NOT NULL" json:"msg_warning_qty"`
	TaskStatus    int    `gorm:"column:task_status;default:1;NOT NULL" json:"task_status"` // 任务状态，1：开启，0：停止
	CreatedAt     int    `gorm:"column:created_at;default:0;NOT NULL" json:"created_at"`
	UpdatedAt     int    `gorm:"column:updated_at;default:0;NOT NULL" json:"updated_at"`
}
