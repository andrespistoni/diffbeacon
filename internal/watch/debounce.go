package watch

import (
	"context"
	"time"
)

const (
	DefaultDebounce  = 150 * time.Millisecond
	DefaultMaxWait   = 500 * time.Millisecond
	DefaultReconcile = 3 * time.Second
)

type Timer interface {
	C() <-chan time.Time
	Stop() bool
	Reset(time.Duration) bool
}

type Clock interface {
	NewTimer(time.Duration) Timer
}

type realClock struct{}

func (realClock) NewTimer(duration time.Duration) Timer {
	return realTimer{timer: time.NewTimer(duration)}
}

type realTimer struct{ timer *time.Timer }

func (t realTimer) C() <-chan time.Time               { return t.timer.C }
func (t realTimer) Stop() bool                        { return t.timer.Stop() }
func (t realTimer) Reset(duration time.Duration) bool { return t.timer.Reset(duration) }

type Debouncer struct {
	Wait    time.Duration
	MaxWait time.Duration
	clock   Clock
}

func NewDebouncer(wait, maxWait time.Duration) Debouncer {
	if wait <= 0 {
		wait = DefaultDebounce
	}
	if maxWait < wait {
		maxWait = DefaultMaxWait
		if maxWait < wait {
			maxWait = wait
		}
	}
	return Debouncer{Wait: wait, MaxWait: maxWait, clock: realClock{}}
}

// Run consolidates a burst after Wait but guarantees output no later than
// MaxWait after the first event in a continuous burst.
func (d Debouncer) Run(ctx context.Context, events <-chan struct{}, output chan<- struct{}) {
	if d.clock == nil {
		d.clock = realClock{}
	}
	var idle, maximum Timer
	var idleC, maximumC <-chan time.Time
	active := false
	for {
		select {
		case <-ctx.Done():
			stopTimer(idle)
			stopTimer(maximum)
			return
		case _, ok := <-events:
			if !ok {
				stopTimer(idle)
				stopTimer(maximum)
				return
			}
			if !active {
				active = true
				idle = d.clock.NewTimer(d.Wait)
				maximum = d.clock.NewTimer(d.MaxWait)
				idleC, maximumC = idle.C(), maximum.C()
				continue
			}
			resetTimer(idle, d.Wait)
		case <-idleC:
			if active {
				emit(ctx, output)
				active = false
				stopTimer(maximum)
				idleC, maximumC = nil, nil
			}
		case <-maximumC:
			if active {
				emit(ctx, output)
				active = false
				stopTimer(idle)
				idleC, maximumC = nil, nil
			}
		}
	}
}

func emit(ctx context.Context, output chan<- struct{}) {
	select {
	case output <- struct{}{}:
	case <-ctx.Done():
	}
}

func stopTimer(timer Timer) {
	if timer == nil || timer.Stop() {
		return
	}
	select {
	case <-timer.C():
	default:
	}
}

func resetTimer(timer Timer, duration time.Duration) {
	stopTimer(timer)
	timer.Reset(duration)
}
