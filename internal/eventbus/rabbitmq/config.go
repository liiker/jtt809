package rabbitmq

import "time"

// Config RabbitMQ 连接配置
type Config struct {
	// 连接地址
	URL string

	// Exchange 配置
	ExchangeName string // 交换机名称
	ExchangeType string // 交换机类型

	// 连接选项
	VHost         string        // 虚拟主机
	MaxReconnect  int           // 最大重连次数，-1 表示无限重连
	ReconnectDelay time.Duration // 重连延迟

	// 发布选项
	Mandatory    bool // 是否启用 mandatory 模式
	Immediate    bool // 是否启用 immediate 模式
	DeliveryMode uint8 // 投递模式：1=非持久化，2=持久化

	// 重试配置
	RetryMaxAttempts  int           // 最大重试次数
	RetryInitialDelay time.Duration // 初始重试延迟
	RetryMaxDelay     time.Duration // 最大重试延迟
	RetryQueueSize    int           // 重试队列缓冲大小
}

// DefaultConfig 返回默认配置
func DefaultConfig() Config {
	return Config{
		URL:               "amqp://guest:guest@localhost:5672/",
		VHost:             "/",
		ExchangeName:      "jtt809.events",
		ExchangeType:      "topic",
		MaxReconnect:      -1, // 无限重连
		ReconnectDelay:    5 * time.Second,
		DeliveryMode:      2, // 持久化
		Mandatory:         false,
		Immediate:         false,
		RetryMaxAttempts:  3,
		RetryInitialDelay: 1 * time.Second,
		RetryMaxDelay:     30 * time.Second,
		RetryQueueSize:    1000,
	}
}
