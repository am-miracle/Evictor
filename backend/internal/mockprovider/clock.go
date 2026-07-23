package mockprovider

import (
	"sync"
	"time"
)

type Clock interface {
	Now() time.Time
}

type RealClock struct{}

func (RealClock) Now() time.Time {
	return time.Now()
}

type VirtualClock struct {
	mu  sync.Mutex
	now time.Time
}

func NewVirtualClock(start time.Time) *VirtualClock {
	return &VirtualClock{now: start}
}

func (vc *VirtualClock) Now() time.Time {
	vc.mu.Lock()
	defer vc.mu.Unlock()

	return vc.now
}

func (vc *VirtualClock) Advance(d time.Duration) {
	vc.mu.Lock()
	defer vc.mu.Unlock()

	vc.now = vc.now.Add(d)
}
