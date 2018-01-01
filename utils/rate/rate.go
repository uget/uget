package rate

import (
	"time"
)

type smoothRate struct {
	sizes      []int64
	intervals  []time.Duration
	smoothness int
	lastTime   time.Time
}

// SmoothRate returns a smooth rater that tracks the speed
func SmoothRate(smoothness uint) Rater {
	return &smoothRate{
		sizes:      make([]int64, 1, smoothness+1),
		intervals:  make([]time.Duration, 1, smoothness+1),
		smoothness: int(smoothness),
		lastTime:   time.Now(),
	}
}

func (sr *smoothRate) Add(progress int64) {
	sr.sizes[len(sr.sizes)-1] += progress
}

func (sr *smoothRate) increment() {
	if len(sr.sizes) > sr.smoothness {
		sr.sizes = sr.sizes[1:]
		sr.intervals = sr.intervals[1:]
	}
	sr.sizes = append(sr.sizes, 0)
	sr.intervals = append(sr.intervals, 0)
	sr.lastTime = time.Now()
}

func (sr *smoothRate) Rate() BPS {
	defer sr.increment()
	sr.intervals[len(sr.intervals)-1] = time.Now().Sub(sr.lastTime)
	return rateFor(sr.sizes, sr.intervals)
}

func rateFor(sizes []int64, intervals []time.Duration) BPS {
	// assert len(sizes) == len(intervals)!!!
	length := len(sizes)
	speeds := make([]float64, length)
	for i, size := range sizes {
		speeds[i] = float64(size) / intervals[i].Seconds()
	}
	speed := 0.0
	significance := 1.0
	for _, s := range speeds {
		speed = (s + speed*significance) / (significance + 1.0)
		significance += 0.5
	}
	return BPS(speed)
}

// BPS - bytes per second
type BPS float64

// Float returns the number of bytes per second
func (b BPS) Float() float64 {
	return float64(b)
}

// BPS returns the number of bytes per second
func (b BPS) BPS() float64 {
	return b.Float()
}

// KBPS returns the number of kilobytes per second
func (b BPS) KBPS() float64 {
	return b.BPS() / 1000
}

// MBPS returns the number of megabytes per second
func (b BPS) MBPS() float64 {
	return b.KBPS() / 1000
}

// GBPS returns the number of gigabytes per second
func (b BPS) GBPS() float64 {
	return b.MBPS() / 1000
}

// Rater tracks download speed
type Rater interface {
	Add(int64)
	Rate() BPS
}
