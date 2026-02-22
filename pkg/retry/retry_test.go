package retry

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func fastCfg() Config {
	return Config{
		MaxRetries:  3,
		InitialWait: 1 * time.Millisecond,
		MaxWait:     4 * time.Millisecond,
		Retryable:   func(err error) bool { return true },
	}
}

func TestDo_SuccessFirstTry(t *testing.T) {
	calls := 0
	err := Do(context.Background(), fastCfg(), func() error {
		calls++
		return nil
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

func TestDo_RetryThenSucceed(t *testing.T) {
	calls := 0
	err := Do(context.Background(), fastCfg(), func() error {
		calls++
		if calls < 3 {
			return errors.New("transient")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
}

func TestDo_AllRetriesExhausted(t *testing.T) {
	calls := 0
	err := Do(context.Background(), fastCfg(), func() error {
		calls++
		return errors.New("persistent")
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// 1 initial + 3 retries = 4
	if calls != 4 {
		t.Fatalf("expected 4 calls, got %d", calls)
	}
}

func TestDo_NonRetryableStopsImmediately(t *testing.T) {
	cfg := fastCfg()
	cfg.Retryable = func(err error) bool { return false }

	calls := 0
	err := Do(context.Background(), cfg, func() error {
		calls++
		return errors.New("non-retryable")
	})
	if err == nil || err.Error() != "non-retryable" {
		t.Fatalf("expected non-retryable error, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

func TestDo_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	err := Do(ctx, fastCfg(), func() error {
		calls++
		if calls == 1 {
			cancel()
		}
		return errors.New("fail")
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestDoWithResult_ReturnsValue(t *testing.T) {
	val, err := DoWithResult(context.Background(), fastCfg(), func() (string, error) {
		return "hello", nil
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if val != "hello" {
		t.Fatalf("expected 'hello', got %q", val)
	}
}

func TestDoWithResult_RetryThenSucceed(t *testing.T) {
	calls := 0
	val, err := DoWithResult(context.Background(), fastCfg(), func() (int, error) {
		calls++
		if calls < 2 {
			return 0, errors.New("transient")
		}
		return 42, nil
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if val != 42 {
		t.Fatalf("expected 42, got %d", val)
	}
}

func TestDefaultRetryable(t *testing.T) {
	tests := []struct {
		err  error
		want bool
	}{
		{nil, false},
		{context.Canceled, false},
		{context.DeadlineExceeded, false},
		{fmt.Errorf("status code: 500"), true},
		{fmt.Errorf("502 bad gateway"), true},
		{fmt.Errorf("503 service unavailable"), true},
		{fmt.Errorf("status code: 400"), false},
		{fmt.Errorf("connection refused"), true},
		{fmt.Errorf("connection reset"), true},
		{fmt.Errorf("unexpected EOF"), true},
		{fmt.Errorf("some unknown error"), false},
	}
	for _, tt := range tests {
		got := DefaultRetryable(tt.err)
		if got != tt.want {
			label := "<nil>"
			if tt.err != nil {
				label = tt.err.Error()
			}
			t.Errorf("DefaultRetryable(%q) = %v, want %v", label, got, tt.want)
		}
	}
}
