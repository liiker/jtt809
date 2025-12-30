package eventbus

import (
	"context"
	"log/slog"

	"github.com/zboyco/jtt809/internal/eventbus/events"
	"github.com/zboyco/jtt809/internal/eventbus/rabbitmq"
)

// EventBus 事件总线接口
type EventBus interface {
	Publish(eventType events.EventType, data []byte) error
	PublishAsync(eventType events.EventType, data []byte)
	Close() error
}

// rabbitEventBus RabbitMQ 实现
type rabbitEventBus struct {
	publisher *rabbitmq.Publisher
	logger    *slog.Logger
}

// New 创建事件总线
func New(cfg rabbitmq.Config, logger *slog.Logger) (EventBus, error) {
	bus := &rabbitEventBus{
		publisher: rabbitmq.NewPublisher(cfg, logger),
		logger:    logger,
	}

	ctx := context.Background()
	if err := bus.publisher.Start(ctx); err != nil {
		return nil, err
	}

	return bus, nil
}

// Publish 发布事件
func (b *rabbitEventBus) Publish(eventType events.EventType, data []byte) error {
	routingKey := string(eventType)
	return b.publisher.Publish(context.Background(), routingKey, data)
}

// PublishAsync 异步发布事件
func (b *rabbitEventBus) PublishAsync(eventType events.EventType, data []byte) {
	routingKey := string(eventType)
	b.publisher.PublishAsync(routingKey, data)
}

// Close 关闭事件总线
func (b *rabbitEventBus) Close() error {
	return b.publisher.Close()
}
