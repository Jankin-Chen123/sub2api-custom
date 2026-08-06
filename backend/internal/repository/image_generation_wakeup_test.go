package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestImageGenerationWakeupPublishesOnlyJobIDsAcrossSubscribers(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	wakeup := NewImageGenerationWakeup(client)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	received := make(chan string, 1)
	errCh := make(chan error, 1)
	go func() {
		errCh <- wakeup.SubscribeImageGenerationWakeups(ctx, func(jobID string) {
			select {
			case received <- jobID:
			default:
			}
		})
	}()

	// Publishing until the subscription handshake is complete avoids relying
	// on a scheduler-specific sleep while retaining the real Redis protocol.
	deadline := time.After(2 * time.Second)
	for {
		if err := wakeup.PublishImageGenerationWakeup(context.Background(), "imgjob_wakeup_test"); err != nil {
			t.Fatalf("publish wakeup: %v", err)
		}
		select {
		case got := <-received:
			require.Equal(t, "imgjob_wakeup_test", got)
			cancel()
			require.ErrorIs(t, <-errCh, context.Canceled)
			return
		case <-deadline:
			t.Fatal("subscriber did not receive image wakeup")
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}
}

func TestImageGenerationWakeupBroadcastsToIndependentClients(t *testing.T) {
	server := miniredis.RunT(t)
	clientA := redis.NewClient(&redis.Options{Addr: server.Addr()})
	clientB := redis.NewClient(&redis.Options{Addr: server.Addr()})
	publisher := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() {
		_ = clientA.Close()
		_ = clientB.Close()
		_ = publisher.Close()
	})
	wakeupA := NewImageGenerationWakeup(clientA)
	wakeupB := NewImageGenerationWakeup(clientB)
	wakeupPublisher := NewImageGenerationWakeup(publisher)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	receivedA := make(chan string, 1)
	receivedB := make(chan string, 1)
	errCh := make(chan error, 2)
	go func() {
		errCh <- wakeupA.SubscribeImageGenerationWakeups(ctx, func(jobID string) {
			select {
			case receivedA <- jobID:
			default:
			}
		})
	}()
	go func() {
		errCh <- wakeupB.SubscribeImageGenerationWakeups(ctx, func(jobID string) {
			select {
			case receivedB <- jobID:
			default:
			}
		})
	}()

	deadline := time.After(2 * time.Second)
	gotA, gotB := false, false
	for !gotA || !gotB {
		require.NoError(t, wakeupPublisher.PublishImageGenerationWakeup(context.Background(), "imgjob_broadcast_test"))
		select {
		case jobID := <-receivedA:
			require.Equal(t, "imgjob_broadcast_test", jobID)
			gotA = true
		case jobID := <-receivedB:
			require.Equal(t, "imgjob_broadcast_test", jobID)
			gotB = true
		case <-deadline:
			t.Fatal("independent Redis subscribers did not both receive the wakeup")
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}
	cancel()
	require.ErrorIs(t, <-errCh, context.Canceled)
	require.ErrorIs(t, <-errCh, context.Canceled)
}

func TestImageGenerationWakeupWithoutRedisIsOptional(t *testing.T) {
	wakeup := NewImageGenerationWakeup(nil)
	require.ErrorIs(t, wakeup.PublishImageGenerationWakeup(context.Background(), "imgjob_test"), service.ErrImageGenerationWakeupUnavailable)
	require.ErrorIs(t, wakeup.SubscribeImageGenerationWakeups(context.Background(), func(string) {}), service.ErrImageGenerationWakeupUnavailable)
}

func TestImageGenerationWakeupSubscriptionStopsOnContextCancellation(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	wakeup := NewImageGenerationWakeup(client)
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- wakeup.SubscribeImageGenerationWakeups(ctx, func(string) {})
	}()
	cancel()

	select {
	case err := <-errCh:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(2 * time.Second):
		t.Fatal("subscription did not stop after context cancellation")
	}
}
