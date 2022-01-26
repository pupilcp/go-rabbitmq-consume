package dao

import (
	"github.com/pupilcp/go-rabbitmq-consume/global"
	"github.com/pupilcp/go-rabbitmq-consume/model"
)

type mqTaskDao struct {
}

var MqTaskDao = mqTaskDao{}

func (d *mqTaskDao) GetList(status int) []model.MqTask {
	var taskList []model.MqTask
	command := global.DB.Model(model.MqTask{}).Select("*")
	if status != -1 {
		command = command.Where("task_status=?", status)
	}
	command.Scan(&taskList)
	return taskList
}

func (d *mqTaskDao) AddTask(data model.MqTask) error {
	global.DB.Model(model.MqTask{}).Create(&data)
	if global.DB.RowsAffected > 0 {
		return nil
	}

	return global.DB.Error
}
