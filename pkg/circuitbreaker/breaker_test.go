package circuitbreaker

import (
	"errors"
	"testing"
	"time"
)

var errFail = errors.New("fail")

func succeedFn() error { return nil }
func failFn() error    { return errFail }

func newTestBreaker() *CircuitBreaker {
	return New("test", WithMaxFailures(3), WithResetTimeout(10*time.Millisecond))
}

func TestStartsClosed(t *testing.T) {
	cb := newTestBreaker()
	if cb.State() != StateClosed {
		t.Fatal("expected StateClosed")
	}
}

func TestSuccessKeepsClosed(t *testing.T) {
	cb := newTestBreaker()
	for i := 0; i < 5; i++ {
		if err := cb.Execute(succeedFn); err != nil {
			t.Fatalf("call %d: unexpected error: %v", i, err)
		}
	}
	if cb.State() != StateClosed {
		t.Fatal("expected StateClosed after successful calls")
	}
}

func TestConsecutiveFailuresOpen(t *testing.T) {
	cb := newTestBreaker()
	for i := 0; i < 3; i++ {
		cb.Execute(failFn)
	}
	if cb.State() != StateOpen {
		t.Fatalf("expected StateOpen, got %d", cb.State())
	}
}

func TestOpenReturnsError(t *testing.T) {
	cb := newTestBreaker()
	for i := 0; i < 3; i++ {
		cb.Execute(failFn)
	}
	err := cb.Execute(succeedFn)
	if !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen, got %v", err)
	}
}

func TestTransitionsToHalfOpen(t *testing.T) {
	cb := newTestBreaker()
	for i := 0; i < 3; i++ {
		cb.Execute(failFn)
	}
	time.Sleep(15 * time.Millisecond)

	if err := cb.Execute(succeedFn); err != nil {
		t.Fatalf("expected success in half-open, got %v", err)
	}
	if cb.State() != StateClosed {
		t.Fatalf("expected StateClosed after half-open success, got %d", cb.State())
	}
}

func TestHalfOpenFailureReopens(t *testing.T) {
	cb := newTestBreaker()
	for i := 0; i < 3; i++ {
		cb.Execute(failFn)
	}
	time.Sleep(15 * time.Millisecond)

	cb.Execute(failFn)
	if cb.State() != StateOpen {
		t.Fatal("expected StateOpen after half-open failure")
	}
}
