package rabbitmq

import (
	"context"
	"log/slog"
	"math"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// RetryTask 重试任务
type RetryTask struct {
	RoutingKey string
	Data       []byte
	Attempts   int
	NextRetry  time.Time
}

// RetryQueue 异步重试队列
type RetryQueue struct {
	cfg    Config
	client *Client
	queue  chan *RetryTask
	wg     sync.WaitGroup
	done   chan struct{}
	logger *slog.Logger
}

// NewRetryQueue 创建重试队列
func NewRetryQueue(cfg Config, client *Client, logger *slog.Logger) *RetryQueue {
	return &RetryQueue{
		cfg:    cfg,
		client: client,
		queue:  make(chan *RetryTask, cfg.RetryQueueSize),
		done:   make(chan struct{}),
		logger: logger,
	}
}

// Start 启动重试队列处理
func (rq *RetryQueue) Start(ctx context.Context) {
	rq.wg.Add(1)
	go rq.processRetry(ctx)
}

// processRetry 处理重试任务
func (rq *RetryQueue) processRetry(ctx context.Context) {
	defer rq.wg.Done()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	pendingTasks := make([]*RetryTask, 0)

	for {
		select {
		case <-ctx.Done():
			return
		case <-rq.done:
			return
		case task := <-rq.queue:
			pendingTasks = append(pendingTasks, task)
		case <-ticker.C:
			now := time.Now()
			remaining := make([]*RetryTask, 0, len(pendingTasks))

			for _, task := range pendingTasks {
				if now.Before(task.NextRetry) {
					remaining = append(remaining, task)
					continue
				}

				// 尝试发布
				err := rq.client.PublishWithContext(ctx, task.RoutingKey, task.Data)
				if err != nil {
					task.Attempts++
					if task.Attempts < rq.cfg.RetryMaxAttempts {
						task.NextRetry = now.Add(rq.calculateBackoff(task.Attempts))
						remaining = append(remaining, task)
						rq.logger.Warn("publish retry failed",
							"routing_key", task.RoutingKey,
							"attempt", task.Attempts,
							"err", err)
					} else {
						rq.logger.Error("publish max retries exceeded",
							"routing_key", task.RoutingKey,
							"attempts", task.Attempts,
							"err", err)
					}
				} else {
					rq.logger.Debug("publish retry success",
						"routing_key", task.RoutingKey,
						"attempt", task.Attempts)
				}
			}

			pendingTasks = remaining
		}
	}
}

// calculateBackoff 计算指数退避延迟
func (rq *RetryQueue) calculateBackoff(attempts int) time.Duration {
	delay := rq.cfg.RetryInitialDelay * time.Duration(math.Pow(2, float64(attempts-1)))
	if delay > rq.cfg.RetryMaxDelay {
		delay = rq.cfg.RetryMaxDelay
	}
	return delay
}

// Enqueue 将任务加入重试队列
func (rq *RetryQueue) Enqueue(routingKey string, data []byte) {
	task := &RetryTask{
		RoutingKey: routingKey,
		Data:       data,
		Attempts:   0,
		NextRetry:  time.Now(),
	}

	select {
	case rq.queue <- task:
		rq.logger.Debug("task enqueued for retry", "routing_key", routingKey)
	default:
		rq.logger.Warn("retry queue full, dropping task", "routing_key", routingKey)
	}
}

// Close 关闭重试队列
func (rq *RetryQueue) Close() {
	close(rq.done)
	rq.wg.Wait()
}

// ensure amqp.Publishing is imported
var _ = amqp.Publishing{}
