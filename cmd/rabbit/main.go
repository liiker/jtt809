package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	var (
		showHelp         = flag.Bool("help", false, "显示帮助信息")
		rabbitmqURL      = flag.String("url", "amqp://guest:guest@localhost:5672/", "RabbitMQ 连接 URL")
		exchangeName     = flag.String("exchange", "jtt809.events", "交换机名称")
		queueName        = flag.String("queue", "", "队列名称，为空则自动生成")
		routingKey       = flag.String("key", "#", "路由键，# 表示订阅所有消息")
		exclusive        = flag.Bool("exclusive", false, "是否创建独占队列")
		autoDelete       = flag.Bool("auto-delete", false, "是否自动删除队列")
	)
	flag.Parse()

	if *showHelp {
		printUsage()
		os.Exit(0)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	slog.SetDefault(logger)

	// 连接 RabbitMQ
	conn, err := amqp.Dial(*rabbitmqURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "连接 RabbitMQ 失败: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	logger.Info("已连接到 RabbitMQ", "url", *rabbitmqURL)

	// 创建 channel
	ch, err := conn.Channel()
	if err != nil {
		fmt.Fprintf(os.Stderr, "创建 channel 失败: %v\n", err)
		os.Exit(1)
	}
	defer ch.Close()

	// 声明交换机
	err = ch.ExchangeDeclare(
		*exchangeName,
		"topic",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "声明交换机失败: %v\n", err)
		os.Exit(1)
	}
	logger.Info("交换机已声明", "name", *exchangeName)

	// 声明队列
	queue := *queueName
	if queue == "" {
		var q amqp.Queue
		q, err = ch.QueueDeclare(
			"",    // 自动生成队列名
			false, // 非持久化
			*exclusive,
			*autoDelete,
			false,
			nil,
		)
		if err != nil {
			fmt.Fprintf(os.Stderr, "声明队列失败: %v\n", err)
			os.Exit(1)
		}
		queue = q.Name
	} else {
		_, err = ch.QueueDeclare(
			queue,
			true,  // 持久化
			false,
			*autoDelete,
			false,
			nil,
		)
		if err != nil {
			fmt.Fprintf(os.Stderr, "声明队列失败: %v\n", err)
			os.Exit(1)
		}
	}
	logger.Info("队列已声明", "name", queue, "routing_key", *routingKey)

	// 绑定队列到交换机
	err = ch.QueueBind(
		queue,
		*routingKey,
		*exchangeName,
		false,
		nil,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "绑定队列失败: %v\n", err)
		os.Exit(1)
	}
	logger.Info("队列已绑定", "queue", queue, "exchange", *exchangeName, "key", *routingKey)

	// 消费消息
	msgs, err := ch.Consume(
		queue,
		"",    // consumer tag
		false, // auto-ack (手动确认)
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "注册消费者失败: %v\n", err)
		os.Exit(1)
	}

	logger.Info("开始消费消息，按 Ctrl+C 退出")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		for d := range msgs {
			slog.Info("收到消息",
				"routing_key", d.RoutingKey,
				"content_type", d.ContentType,
				"body", string(d.Body),
			)
			d.Ack(false) // 确认消息
		}
	}()

	<-ctx.Done()
	logger.Info("正在关闭...")
}

// 打印使用帮助
func printUsage() {
	fmt.Println(`
╔════════════════════════════════════════════════════════════╗
║           RabbitMQ 消息订阅测试客户端                       ║
╚════════════════════════════════════════════════════════════╝

支持的 Routing Key:
  login                       - 平台登录事件
  vehicle.registration        - 车辆注册事件
  vehicle.location            - 车辆定位事件
  vehicle.location.supplementary - 批量定位事件
  video.response              - 视频应答事件
  authorize                   - 视频鉴权事件
  monitor.startup.ack         - 监控开启应答
  monitor.end.ack             - 监控结束应答
  warn.adpt.info              - 报警信息事件

示例:
  # 订阅所有消息
  go run cmd/rabbit/main.go

  # 只订阅车辆定位消息
  go run cmd/rabbit/main.go -key "vehicle.location"

  # 订阅所有 vehicle 开头的消息
  go run cmd/rabbit/main.go -key "vehicle.*"

  # 使用持久化队列
  go run cmd/rabbit/main.go -queue "my-queue" -key "#"

  # 连接到远程 RabbitMQ
  go run cmd/rabbit/main.go -url "amqp://user:pass@192.168.1.100:5672/"
`)
}
