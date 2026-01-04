package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/zboyco/jtt809/internal/eventbus"
	"github.com/zboyco/jtt809/internal/eventbus/events"
	"github.com/zboyco/jtt809/internal/eventbus/rabbitmq"
	"github.com/zboyco/jtt809/pkg/jtt1078"
	"github.com/zboyco/jtt809/pkg/jtt809"
	"github.com/zboyco/jtt809/pkg/server"
)

func main() {
	cfg, rmqCfg, err := parseConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse config: %v\n", err)
		os.Exit(2)
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	slog.SetDefault(logger)

	gateway, err := server.NewJT809Gateway(cfg, jtt1078.NewVideoServer(""))
	if err != nil {
		fmt.Fprintf(os.Stderr, "init gateway: %v\n", err)
		os.Exit(2)
	}

	// 初始化事件总线（如果配置了 RabbitMQ）
	var eventBus eventbus.EventBus
	if rmqCfg != nil {
		eventBus, err = eventbus.New(*rmqCfg, logger)
		if err != nil {
			fmt.Fprintf(os.Stderr, "init eventbus: %v\n", err)
			os.Exit(2)
		}
		slog.Info("eventbus initialized", "url", rmqCfg.URL)
	}

	// 设置回调函数，用于处理各类消息
	gateway.SetCallbacks(&server.Callbacks{
		OnLogin: func(userID uint32, req *jtt809.LoginRequest, resp *jtt809.LoginResponse) {
			slog.Info("【业务回调】平台登录",
				"user_id", userID,
				"result", resp.Result,
				"down_link", fmt.Sprintf("%s:%d", req.DownLinkIP, req.DownLinkPort))
			// 发布事件到 RabbitMQ
			if eventBus != nil {
				eventType, data, err := events.MarshalLogin(userID, req, resp)
				if err == nil {
					eventBus.PublishAsync(eventType, data)
				}
			}
		},
		OnVehicleRegistration: func(userID uint32, plate string, color jtt809.PlateColor, reg *server.VehicleRegistration) {
			slog.Info("【业务回调】车辆注册",
				"user_id", userID,
				"plate", plate,
				"color", color,
				"terminal_id", reg.TerminalID)
			// 发布事件到 RabbitMQ
			if eventBus != nil {
				eventType, data, err := events.MarshalVehicleRegistration(userID, plate, color, reg)
				if err == nil {
					eventBus.PublishAsync(eventType, data)
				}
			}
		},
		OnVehicleLocation: func(userID uint32, plate string, color jtt809.PlateColor, pos *jtt809.VehiclePosition, gnss *jtt809.GNSSData) {
			if gnss != nil {
				slog.Info("【业务回调】车辆定位",
					"user_id", userID,
					"plate", plate,
					"lon", gnss.Longitude,
					"lat", gnss.Latitude,
					"speed", gnss.Speed)
			}
			// 发布事件到 RabbitMQ
			if eventBus != nil && gnss != nil {
				eventType, data, err := events.MarshalVehicleLocation(userID, plate, color, pos, gnss)
				if err == nil {
					eventBus.PublishAsync(eventType, data)
				}
			}
		},
		OnVehicleLocationSupplementary: func(userID uint32, plate string, color jtt809.PlateColor, gnss []jtt809.GNSSData) {
			slog.Info("【业务回调】批量定位",
				"user_id", userID,
				"plate", plate,
				"count", len(gnss))
			// 发布事件到 RabbitMQ
			if eventBus != nil {
				eventType, data, err := events.MarshalVehicleLocationSupplementary(userID, plate, color, gnss)
				if err == nil {
					eventBus.PublishAsync(eventType, data)
				}
			}
		},
		OnVideoResponse: func(userID uint32, plate string, color jtt809.PlateColor, videoAck *server.VideoAckState) {
			slog.Info("【业务回调】视频应答",
				"user_id", userID,
				"plate", plate,
				"server", fmt.Sprintf("%s:%d", videoAck.ServerIP, videoAck.ServerPort),
				"result", videoAck.Result)
			// 发布事件到 RabbitMQ
			if eventBus != nil {
				eventType, data, err := events.MarshalVideoResponse(userID, plate, color, videoAck)
				if err == nil {
					eventBus.PublishAsync(eventType, data)
				}
			}
		},
		OnAuthorize: func(userID uint32, platformID string, authorizeCode string) {
			slog.Info("【业务回调】视频鉴权",
				"user_id", userID,
				"platform_id", platformID,
				"auth_code", authorizeCode)
			// 发布事件到 RabbitMQ
			if eventBus != nil {
				eventType, data, err := events.MarshalAuthorize(userID, platformID, authorizeCode)
				if err == nil {
					eventBus.PublishAsync(eventType, data)
				}
			}
		},
		OnMonitorStartupAck: func(userID uint32, plate string, color jtt809.PlateColor) {
			slog.Info("【业务回调】车辆监控开启应答",
				"user_id", userID,
				"plate", plate,
				"color", color)
			// 发布事件到 RabbitMQ
			if eventBus != nil {
				eventType, data, err := events.MarshalMonitorStartupAck(userID, plate, color)
				if err == nil {
					eventBus.PublishAsync(eventType, data)
				}
			}
		},
		OnMonitorEndAck: func(userID uint32, plate string, color jtt809.PlateColor) {
			slog.Info("【业务回调】车辆监控结束应答",
				"user_id", userID,
				"plate", plate,
				"color", color)
			// 发布事件到 RabbitMQ
			if eventBus != nil {
				eventType, data, err := events.MarshalMonitorEndAck(userID, plate, color)
				if err == nil {
					eventBus.PublishAsync(eventType, data)
				}
			}
		},
		OnWarnMsgAdptInfo: func(userID uint32, info *jtt809.WarnMsgAdptInfo) {
			slog.Info("【业务回调】报警信息适配",
				"user_id", userID,
				"type", info.WarnType,
				"info", info.InfoContent)
			// 发布事件到 RabbitMQ
			if eventBus != nil {
				eventType, data, err := events.MarshalWarnMsgAdptInfo(userID, info)
				if err == nil {
					eventBus.PublishAsync(eventType, data)
				}
			}
		},
	})

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := gateway.Start(ctx); err != nil && err != context.Canceled {
		slog.Error("gateway stopped with error", "err", err)
	}

	// 关闭事件总线
	if eventBus != nil {
		eventBus.Close()
	}
}

// parseConfig 解析命令行参数，返回标准化配置。
func parseConfig() (server.Config, *rabbitmq.Config, error) {
	var (
		mainAddr  = flag.String("main", ":10709", "主链路监听地址，格式 host:port")
		httpAddr  = flag.String("http", ":18080", "管理与调度 HTTP 地址")
		idleSec   = flag.Int("idle", 300, "连接空闲超时时间，单位秒，<=0 表示不超时")
		accountFS server.MultiAccountFlag

		// RabbitMQ 配置参数
		rabbitmqURL            = flag.String("rabbitmq-url", "", "RabbitMQ 连接 URL，如: amqp://user:pass@localhost:5672/")
		rabbitmqExchange       = flag.String("rabbitmq-exchange", "jtt809.events", "RabbitMQ 交换机名称")
		rabbitmqVHost          = flag.String("rabbitmq-vhost", "/", "RabbitMQ 虚拟主机")
		rabbitmqMaxReconnect   = flag.Int("rabbitmq-max-reconnect", -1, "RabbitMQ 最大重连次数，-1 表示无限重连")
		rabbitmqReconnectDelay = flag.Int("rabbitmq-reconnect-delay", 5, "RabbitMQ 重连延迟，单位秒")
		rabbitmqRetryAttempts  = flag.Int("rabbitmq-retry-attempts", 3, "RabbitMQ 发布重试次数")
	)
	flag.Var(&accountFS, "account", "下级平台账号，格式 userID:password:gnssCenterID[:allowIPs]，allowIPs 逗号分隔，可重复指定")
	flag.Parse()

	cfg := server.Config{
		MainListen: *mainAddr,
		HTTPListen: *httpAddr,
		IdleTimeout: func() time.Duration {
			if *idleSec <= 0 {
				return 0
			}
			return time.Duration(*idleSec) * time.Second
		}(),
	}

	cfg.Accounts = accountFS

	// RabbitMQ 配置
	var rmqCfg *rabbitmq.Config
	if *rabbitmqURL != "" {
		rmqCfg = &rabbitmq.Config{
			URL:               *rabbitmqURL,
			VHost:             *rabbitmqVHost,
			ExchangeName:      *rabbitmqExchange,
			ExchangeType:      "topic",
			MaxReconnect:      *rabbitmqMaxReconnect,
			ReconnectDelay:    time.Duration(*rabbitmqReconnectDelay) * time.Second,
			DeliveryMode:      2, // 持久化
			RetryMaxAttempts:  *rabbitmqRetryAttempts,
			RetryInitialDelay: 1 * time.Second,
			RetryMaxDelay:     30 * time.Second,
			RetryQueueSize:    1000,
		}
	}

	return cfg, rmqCfg, nil
}
