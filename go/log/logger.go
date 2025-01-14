package log

import (
	"github.com/natefinch/lumberjack"
	"github.com/sirupsen/logrus"
	"io"
	"os"
	"sync"
	"time"
)

var (
	Logger *logrus.Logger
	once   = sync.Once{}
)

func init() {
	GetLogger()
}

type MyLogrus struct {
	logger *logrus.Logger
}

// GetLogger get only one instance
func GetLogger() *logrus.Logger {
	once.Do(func() {
		// 创建一个新的 logrus 实例
		Logger = logrus.New()

		// 设置日志级别
		Logger.SetLevel(logrus.InfoLevel)

		// 创建日志切割器
		logWriter := &lumberjack.Logger{
			Filename:   "log/wd-log.log",
			MaxSize:    20,   // 单个日志文件最大大小（MB）
			MaxBackups: 3,    // 保留旧文件的最大数量
			MaxAge:     7,    // 保留旧文件的最大天数
			Compress:   true, // 压缩旧文件
		}

		writer := io.MultiWriter(logWriter, os.Stdout)

		// 设置日志输出为文件
		Logger.SetOutput(writer)

		// 自定义日志格式
		logFormatter := &logrus.TextFormatter{
			TimestampFormat:  time.DateTime,
			FullTimestamp:    true,
			ForceColors:      true,
			DisableColors:    false,
			PadLevelText:     true,
			QuoteEmptyFields: true,
			FieldMap: logrus.FieldMap{
				logrus.FieldKeyTime:  "@timestamp",
				logrus.FieldKeyMsg:   "message",
				logrus.FieldKeyLevel: "severity",
			},
		}
		Logger.SetFormatter(logFormatter)
	})

	return Logger
}

// InitLog init Logger
func InitLog() *MyLogrus {
	return &MyLogrus{logger: GetLogger()}
}

func (l *MyLogrus) Println(message string) {
	l.logger.Println(message)
}

// Trace level logging. Works like Sprintf.
func (l *MyLogrus) Trace(message string) {
	l.logger.Trace(message)
}

// Debug level logging. Works like Sprintf.
func (l *MyLogrus) Debug(message string) {
	l.logger.Debug(message)
}

// Info level logging. Works like Sprintf.
func (l *MyLogrus) Info(message string) {
	l.logger.Info(message)
}

// Warning level logging. Works like Sprintf.
func (l *MyLogrus) Warning(message string) {
	l.logger.Warning(message)
}

// Error level logging. Works like Sprintf.
func (l *MyLogrus) Error(message string) {
	l.logger.Error(message)
}

// Fatal level logging. Works like Sprintf.
func (l *MyLogrus) Fatal(message string) {
	l.logger.Fatal(message)
	//os.Exit(1)
}

func (l *MyLogrus) Print(message string) {
	l.logger.Print(message)
}
