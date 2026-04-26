package agent

import (
	"math"
	"testing"
)

// quoteStable reports whether the implied mid price is stable between two quotes:
// |cur-prev|/prev * 100 <= maxJitterPct. prev must be > 0.
func quoteStable(prevPrice, curPrice, maxJitterPct float64) bool {
	if prevPrice <= 0 {
		return false
	}
	jitterPct := math.Abs(curPrice-prevPrice) / prevPrice * 100
	return jitterPct <= maxJitterPct
}

func TestQuoteStable(t *testing.T) {
	tests := []struct {
		prev, cur, maxJitter float64
		want                  bool
	}{
		{100, 100, 0.5, true},
		{100, 100.4, 0.5, true},
		{100, 99.6, 0.5, true},
		{100, 100.6, 0.5, false},
		{100, 99.4, 0.5, false},
		{100, 99, 1.0, true},
		{100, 0, 0.5, false},
		{0, 100, 0.5, false},
	}
	for _, tt := range tests {
		got := quoteStable(tt.prev, tt.cur, tt.maxJitter)
		if got != tt.want {
			t.Errorf("quoteStable(%.2f, %.2f, %.2f) = %v, want %v",
				tt.prev, tt.cur, tt.maxJitter, got, tt.want)
		}
	}
}
