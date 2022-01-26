package global

import (
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jinzhu/gorm"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"gopkg.in/natefinch/lumberjack.v2"
	"time"
)

var (
	ServerLogger *log.Logger
	Config       *viper.Viper
	DB           *gorm.DB
)

func SetUp() {
	setConfig()
	setLogger()
	if err := SetDB(); err != nil {
		log.Fatalf("connect db failed: %+v", err)
	}
}

func setLogger() {

	logger := &lumberjack.Logger{
		// 日志输出文件路径
		Filename: Config.Get("log.logPath").(string) + "/" + time.Now().Format("20060102") + ".log",
		// 日志文件最大 size, 单位是 MB
		MaxSize: int(Config.Get("log.maxSize").(int64)), // megabytes
		// 最大过期日志保留的个数
		MaxBackups: int(Config.Get("log.maxBackups").(int64)),
		// 保留过期文件的最大时间间隔,单位是天
		MaxAge: int(Config.Get("log.maxAge").(int64)), //days
		// 是否需要压缩滚动日志, 使用的 gzip 压缩
		Compress: Config.Get("log.compress").(bool), // disabled by default
	}
	ServerLogger = log.New()
	ServerLogger.SetOutput(logger) //调用 logrus 的 SetOutput()函数
}

func setConfig() {
	Config = viper.New()
	Config.SetConfigName("config")
	Config.SetConfigType("toml")
	Config.AddConfigPath("./config")
	err := Config.ReadInConfig()
	if err != nil {
		log.Fatalf("read config failed: %+v", err)
	}
}

func SetDB() error {
	var err error
	DB, err = gorm.Open("mysql",
		fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8&parseTime=true&timeout=%ds",
			Config.Get("mysql.user"),
			Config.Get("mysql.password"),
			Config.Get("mysql.host"),
			Config.Get("mysql.port"),
			Config.Get("mysql.database"),
			int(Config.GetInt64("mysql.timeout")),
		))
	if err != nil {
		errMsg := fmt.Sprintf("db connect err:%+v", err)
		ServerLogger.Error(errMsg)
		return err
	}
	DB.SingularTable(true)
	DB.DB().SetMaxOpenConns(int(Config.Get("mysql.maxOpenConn").(int64)))
	DB.DB().SetMaxIdleConns(int(Config.Get("mysql.maxIdleConn").(int64)))

	return nil
}
