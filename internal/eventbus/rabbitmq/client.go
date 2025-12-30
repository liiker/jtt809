package rabbitmq

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Client RabbitMQ 客户端，封装连接管理和自动重连
type Client struct {
	cfg    Config
	conn   *amqp.Connection
	channel *amqp.Channel

	connMu sync.RWMutex
	done   chan struct{}

	logger *slog.Logger
}

// NewClient 创建新的 RabbitMQ 客户端
func NewClient(cfg Config, logger *slog.Logger) *Client {
	return &Client{
		cfg:    cfg,
		done:   make(chan struct{}),
		logger: logger,
	}
}

// Connect 连接到 RabbitMQ 服务器
func (c *Client) Connect(ctx context.Context) error {
	c.connMu.Lock()
	defer c.connMu.Unlock()

	var err error
	c.conn, err = amqp.DialConfig(c.cfg.URL, amqp.Config{
		Vhost:      c.cfg.VHost,
		Heartbeat:  10 * time.Second,
		Locale:     "en_US",
		Properties: nil,
	})
	if err != nil {
		return fmt.Errorf("dial rabbitmq: %w", err)
	}

	// 创建 channel
	c.channel, err = c.conn.Channel()
	if err != nil {
		c.conn.Close()
		return fmt.Errorf("open channel: %w", err)
	}

	// 声明交换机
	if err := c.declareExchange(); err != nil {
		c.channel.Close()
		c.conn.Close()
		return fmt.Errorf("declare exchange: %w", err)
	}

	// 监听连接关闭事件
	go c.monitorConnection(ctx)

	c.logger.Info("rabbitmq connected", "url", c.cfg.URL, "exchange", c.cfg.ExchangeName)
	return nil
}

// declareExchange 声明交换机
func (c *Client) declareExchange() error {
	return c.channel.ExchangeDeclare(
		c.cfg.ExchangeName,
		c.cfg.ExchangeType,
		true,  // durable
		false, // auto-deleted
		false, // internal
		false, // no-wait
		nil,   // arguments
	)
}

// monitorConnection 监控连接状态，处理自动重连
func (c *Client) monitorConnection(ctx context.Context) {
	connCloseChan := c.conn.NotifyClose(make(chan *amqp.Error, 1))

	select {
	case err := <-connCloseChan:
		if err != nil {
			c.logger.Warn("rabbitmq connection closed", "err", err)
			c.reconnect(ctx)
		}
	case <-ctx.Done():
		return
	case <-c.done:
		return
	}
}

// reconnect 自动重连
func (c *Client) reconnect(ctx context.Context) {
	ticker := time.NewTicker(c.cfg.ReconnectDelay)
	defer ticker.Stop()

	attempts := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.done:
			return
		case <-ticker.C:
			attempts++

			if c.cfg.MaxReconnect > 0 && attempts > c.cfg.MaxReconnect {
				c.logger.Error("max reconnect attempts reached", "attempts", attempts)
				return
			}

			c.logger.Info("attempting to reconnect", "attempt", attempts)

			if err := c.Connect(ctx); err != nil {
				c.logger.Warn("reconnect failed", "attempt", attempts, "err", err)
				continue
			}

			c.logger.Info("reconnect success", "attempt", attempts)
			return
		}
	}
}

// PublishWithContext 发布消息
func (c *Client) PublishWithContext(ctx context.Context, routingKey string, data []byte) error {
	c.connMu.RLock()
	defer c.connMu.RUnlock()

	if c.channel == nil || c.channel.IsClosed() {
		return fmt.Errorf("channel not ready")
	}

	return c.channel.PublishWithContext(
		ctx,
		c.cfg.ExchangeName,
		routingKey,
		c.cfg.Mandatory,
		c.cfg.Immediate,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: c.cfg.DeliveryMode,
			Body:         data,
			Timestamp:    time.Now(),
		},
	)
}

// Close 关闭连接
func (c *Client) Close() error {
	close(c.done)
	c.connMu.Lock()
	defer c.connMu.Unlock()

	if c.channel != nil {
		c.channel.Close()
	}
	if c.conn != nil && !c.conn.IsClosed() {
		return c.conn.Close()
	}
	return nil
}
