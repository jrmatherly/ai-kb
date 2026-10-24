package datasource

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestReadCappedRefusesOverLimitAndDefaultsZero(t *testing.T) {
	got, err := ReadCapped(strings.NewReader("abc"), 3)
	if err != nil || string(got) != "abc" {
		t.Fatalf("at the limit: %q %v", got, err)
	}
	_, err = ReadCapped(strings.NewReader("abcd"), 3)
	if !errors.Is(err, ErrResponseTooLarge) || !strings.Contains(err.Error(), "(3 bytes)") {
		t.Fatalf("over the limit: %v", err)
	}
	got, err = ReadCapped(strings.NewReader(strings.Repeat("x", 1<<10)), 0)
	if err != nil || len(got) != 1<<10 {
		t.Fatalf("zero limit means the default cap: %d %v", len(got), err)
	}
}

func TestSleepContextHonorsCancellationAndNonPositive(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := SleepContext(ctx, time.Hour); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled: %v", err)
	}
	if err := SleepContext(ctx, 0); !errors.Is(err, context.Canceled) {
		t.Fatalf("non-positive still reports a dead context: %v", err)
	}
	if err := SleepContext(context.Background(), 0); err != nil {
		t.Fatalf("non-positive on a live context: %v", err)
	}
	start := time.Now()
	err := SleepContext(context.Background(), 20*time.Millisecond)
	if err != nil || time.Since(start) < 20*time.Millisecond {
		t.Fatalf("waits the full duration: %v after %s", err, time.Since(start))
	}
}
