package router

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jrmatherly/ai-kb/internal/application/service"
	"github.com/jrmatherly/ai-kb/internal/types"
)

func TestSyncTaskExecutorInjectsRetryMetadata(t *testing.T) {
	executor := NewSyncTaskExecutor()
	observed := make(chan [2]int, 1)
	executor.RegisterHandler("test:retry-metadata", func(ctx context.Context, _ *asynq.Task) error {
		retried, maxRetry, ok := types.TaskRetryMetadataFromContext(ctx)
		if !ok {
			observed <- [2]int{-1, -1}
			return nil
		}
		observed <- [2]int{retried, maxRetry}
		return nil
	})

	task := asynq.NewTask("test:retry-metadata", nil)
	if _, err := executor.Enqueue(task, asynq.MaxRetry(3)); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	select {
	case got := <-observed:
		if got != [2]int{0, 3} {
			t.Fatalf("retry metadata = %v, want [0 3]", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for sync task")
	}
}

func TestSyncRetryDelayUsesFixedWikiConflictDelay(t *testing.T) {
	conflict := fmt.Errorf("batch: %w", service.ErrWikiIngestConcurrent)
	for attempt, want := range map[int]time.Duration{1: 15 * time.Second, 4: 15 * time.Second, 9: 15 * time.Second} {
		if got := syncRetryDelay(attempt, conflict); got != want {
			t.Fatalf("wiki conflict attempt %d: delay %s, want %s", attempt, got, want)
		}
	}
	other := errors.New("transient")
	for attempt, want := range map[int]time.Duration{1: 5 * time.Second, 3: 15 * time.Second, 9: 30 * time.Second} {
		if got := syncRetryDelay(attempt, other); got != want {
			t.Fatalf("other error attempt %d: delay %s, want %s", attempt, got, want)
		}
	}
}
