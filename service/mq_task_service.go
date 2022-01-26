package service

import (
	"github.com/pupilcp/go-rabbitmq-consume/model"
	"github.com/pupilcp/go-rabbitmq-consume/model/dao"
)

type mqTaskService struct {
}

var MqTaskService mqTaskService

func (s *mqTaskService) GetTaskList(status int) []model.MqTask {

	return dao.MqTaskDao.GetList(status)
}
