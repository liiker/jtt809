package rabbitmq

import (
	"context"
	"log/slog"
	"time"
)

// Publisher 事件发布器
type Publisher struct {
	client     *Client
	retryQueue *RetryQueue
	logger     *slog.Logger
}

// NewPublisher 创建事件发布器
func NewPublisher(cfg Config, logger *slog.Logger) *Publisher {
	client := NewClient(cfg, logger)

	return &Publisher{
		client: client,
		logger: logger,
	}
}

// Start 启动发布器
func (p *Publisher) Start(ctx context.Context) error {
	if err := p.client.Connect(ctx); err != nil {
		return err
	}

	// 启动重试队列
	p.retryQueue = NewRetryQueue(p.client.cfg, p.client, p.logger)
	p.retryQueue.Start(ctx)

	return nil
}

// Publish 发布事件
func (p *Publisher) Publish(ctx context.Context, routingKey string, data []byte) error {
	err := p.client.PublishWithContext(ctx, routingKey, data)
	if err != nil {
		p.logger.Warn("publish failed, enqueuing retry",
			"routing_key", routingKey,
			"err", err)
		p.retryQueue.Enqueue(routingKey, data)
		return nil
	}
	return nil
}

// PublishAsync 异步发布事件
func (p *Publisher) PublishAsync(routingKey string, data []byte) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = p.Publish(ctx, routingKey, data)
	}()
}

// Close 关闭发布器
func (p *Publisher) Close() error {
	if p.retryQueue != nil {
		p.retryQueue.Close()
	}
	return p.client.Close()
}
