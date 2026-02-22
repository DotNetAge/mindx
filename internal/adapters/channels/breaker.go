package channels

import (
	"mindx/pkg/circuitbreaker"
	"sync"
)

var (
	channelBreakers = make(map[string]*circuitbreaker.CircuitBreaker)
	breakerMu       sync.Mutex
)

// getBreaker returns or creates a circuit breaker for the given channel name.
func getBreaker(name string) *circuitbreaker.CircuitBreaker {
	breakerMu.Lock()
	defer breakerMu.Unlock()

	if cb, ok := channelBreakers[name]; ok {
		return cb
	}

	cb := circuitbreaker.New(name)
	channelBreakers[name] = cb
	return cb
}
