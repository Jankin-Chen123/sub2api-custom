package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const imageGenerationWakeupChannel = "image:generation:wakeup"

type imageGenerationWakeup struct {
	rdb *redis.Client
}

// NewImageGenerationWakeup creates the optional cross-process notifier for
// durable image jobs. PostgreSQL remains authoritative when Redis is down.
func NewImageGenerationWakeup(rdb *redis.Client) service.ImageGenerationWakeup {
	return &imageGenerationWakeup{rdb: rdb}
}

func (w *imageGenerationWakeup) PublishImageGenerationWakeup(ctx context.Context, jobID string) error {
	if w == nil || w.rdb == nil {
		return service.ErrImageGenerationWakeupUnavailable
	}
	jobID = strings.TrimSpace(jobID)
	if jobID == "" {
		return errors.New("image generation wakeup job ID is required")
	}
	return w.rdb.Publish(ctx, imageGenerationWakeupChannel, jobID).Err()
}

func (w *imageGenerationWakeup) SubscribeImageGenerationWakeups(ctx context.Context, handler func(jobID string)) error {
	if w == nil || w.rdb == nil {
		return service.ErrImageGenerationWakeupUnavailable
	}
	if handler == nil {
		return errors.New("image generation wakeup handler is required")
	}

	pubsub := w.rdb.Subscribe(ctx, imageGenerationWakeupChannel)
	defer func() { _ = pubsub.Close() }()
	// PubSub.Receive may be blocked in a network read while the connection is
	// being established. Close the connection on cancellation so Stop cannot
	// wait indefinitely for a Redis handshake that will never complete.
	handshake := make(chan error, 1)
	go func() {
		_, err := pubsub.Receive(ctx)
		handshake <- err
	}()
	select {
	case err := <-handshake:
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("subscribe image generation wakeup: %w", err)
		}
	case <-ctx.Done():
		_ = pubsub.Close()
		<-handshake
		return ctx.Err()
	}
	messages := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case message, ok := <-messages:
			if !ok {
				return errors.New("image generation wakeup pubsub channel closed")
			}
			if message == nil || strings.TrimSpace(message.Payload) == "" {
				continue
			}
			handler(strings.TrimSpace(message.Payload))
		}
	}
}
