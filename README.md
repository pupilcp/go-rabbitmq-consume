# go-rabbitmq-consume
### 简介
使用go多协程同时消费rabbitmq的消息，和业务解耦，通过URL回调的方式，实现消息处理

### 功能实现
1. 使用go多协程消费rabbitmq队列的消息
2. 通过POST回调URL实现消息处理
3. 检测DB连接状态并实现自动重连
4. 检测MQ服务的状态
5. 检测MQ队列消息挤压的数量以及消费者为0的情况
6. 监听消费任务的变化，实现热更新，自动扩展和减少任务、消费者
7. 接入企业微信/钉钉实现消息预警

### 特点
1. 无需重启服务，自动伸缩消费者数量
2. 使用连接池实现连接rabbitmq
3. 使用goroutine+channel实现预警信息的异步发送
4. 使用signal实现平滑停止服务或查询服务状态

### 核心依赖库
1. viper解析toml配置文件
2. gorm操作mysql
3. logrus+lumberjack实现日志的记录以及文件的自动切割
4. resty实现http请求
5. amqp实现操作rabbitmq
6. urfave/cli实现命令启动/停止/查看服务

### 安装前准备
#### 环境要求
1. go版本最好 >=1.15
2. mysql
3. rabbitmq

#### 导入sql数据
将config下的data.sql文件导入到mysql数据库

#### 修改配置文件
复制根目录下的config/config_demo.toml，并重命名为：config.toml，修改配置文件里的参数。具体参数请参考
config_demo.toml说明。

### 安装：
1. git clone https://github.com/pupilcp/go-rabbitmq-consume.git
2. 安装依赖：cd $PATH, go mod tidy
3. 编译：cd $PATH, go build -o main.go，生成server二进制执行文件。

### 使用
1. ./server start 启动服务
2. ./server stop 停止服务
3. ./server status 查看服务状态

### 支持
go

### 其它
如需交流，请邮件联系：310976780@qq.com