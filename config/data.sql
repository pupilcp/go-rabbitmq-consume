CREATE TABLE `mq_task` (
   `id` int(11) NOT NULL AUTO_INCREMENT,
   `task_name` varchar(255) NOT NULL DEFAULT '' COMMENT '任务名称',
   `vhost` varchar(255) NOT NULL DEFAULT '' COMMENT 'vhost名称',
   `exchange_name` varchar(255) NOT NULL DEFAULT '' COMMENT '交换机名称',
   `route_key` varchar(255) NOT NULL DEFAULT '' COMMENT '路由key',
   `queue_name` varchar(255) NOT NULL DEFAULT '' COMMENT '队列名称',
   `push_url` varchar(255) NOT NULL DEFAULT '' COMMENT '推送的url（post方式）',
   `consumer_num` tinyint(1) NOT NULL DEFAULT '0' COMMENT '消费者数',
   `msg_warning_qty` int(11) NOT NULL DEFAULT '0' COMMENT '队列消息预警数量',
   `task_status` tinyint(1) NOT NULL DEFAULT '1' COMMENT '任务状态，1：开启，0：停止',
   `created_at` int(11) NOT NULL DEFAULT '0' COMMENT '任务创建时间',
   `updated_at` int(11) NOT NULL DEFAULT '0' COMMENT '任务更新时间',
   PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='MQ队列消费任务表';