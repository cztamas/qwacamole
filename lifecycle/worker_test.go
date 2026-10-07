package lifecycle

import (
	"testing"
	"time"
)

func TestRetryBoundsAndJitter(t *testing.T) {
	seen := map[time.Duration]bool{}
	for _, n := range []int{0, 1, 2, 7, 100} {
		base := time.Second
		if n == 2 {
			base = 2 * time.Second
		}
		if n >= 7 {
			base = 48 * time.Second
		}
		for range 20 {
			d := RetryDelay(n)
			if d < base || d >= base*5/4 || d >= time.Minute {
				t.Fatal(n, d)
			}
			seen[d] = true
		}
	}
	if len(seen) < 5 {
		t.Fatal("jitter absent")
	}
}
