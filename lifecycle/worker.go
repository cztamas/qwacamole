// Package lifecycle runs the shared expiry-driven binding maintenance worker.
// Integrations own process locks, activation and cancellation.
package lifecycle

import (
	"context"
	"math/rand/v2"
	"time"

	"github.com/cztamas/qwacamole/store"
)

// Event reports maintenance and diagnostic outcomes to the owning integration.
// Observe must return promptly; it must not acquire an integration activation lock.
type Event struct {
	Kind       string
	Err        error
	RetryAfter time.Duration
	Collection store.Collection
	Status     store.Status
}

// Run maintains one exclusively owned manager until cancellation. It never
// extends signed validity and never relaxes the delivery availability gate.
func Run(ctx context.Context, m *store.Manager, observe func(Event)) {
	emit := func(e Event) {
		if observe == nil {
			return
		}
		s, err := m.Status()
		e.Status = s
		observe(e)
		if err != nil {
			observe(Event{Kind: "diagnostics", Err: err})
		}
	}
	gc := time.NewTicker(m.CollectionInterval())
	defer gc.Stop()
	var retryAt time.Time
	failures := 0
	for {
		due := m.NextRefresh()
		if retryAt.After(due) {
			due = retryAt
		}
		delay := time.Until(due)
		if delay < time.Millisecond {
			delay = time.Millisecond
		}
		t := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case <-m.Changed():
			t.Stop()
			retryAt = time.Time{}
			failures = 0
			m.Maintenance(retryAt, failures, nil)
		case now := <-gc.C:
			t.Stop()
			result, err := m.Collect(now)
			emit(Event{Kind: "collection", Err: err, Collection: result})
		case <-t.C:
			if err := m.Refresh(); err != nil {
				failures++
				delay := RetryDelay(failures)
				remaining := time.Until(time.Unix(m.Snapshot().Expires, 0))
				if remaining > 2*time.Second && delay > remaining/2 {
					delay = remaining / 2
				}
				retryAt = time.Now().Add(delay)
				m.Maintenance(retryAt, failures, err)
				emit(Event{Kind: "refresh_failed", Err: err, RetryAfter: delay})
			} else {
				m.Maintenance(time.Time{}, 0, nil)
				if failures > 0 {
					emit(Event{Kind: "refresh_recovered"})
				}
				failures = 0
				retryAt = time.Time{}
			}
		}
	}
}

// RetryDelay adds up to 25% jitter to exponential retry, always below one minute.
func RetryDelay(failures int) time.Duration {
	if failures < 1 {
		failures = 1
	}
	if failures > 7 {
		failures = 7
	}
	base := time.Second * time.Duration(1<<(failures-1))
	if base > 48*time.Second {
		base = 48 * time.Second
	}
	return time.Duration(float64(base) * (1 + rand.Float64()/4))
}
