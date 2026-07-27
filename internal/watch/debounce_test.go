package watch

import (
	"context"
	"sync"
	"testing"
	"time"
)

type fakeClock struct {
	mu     sync.Mutex
	now    time.Time
	timers []*fakeTimer
}

type fakeTimer struct {
	clock    *fakeClock
	channel  chan time.Time
	deadline time.Time
	active   bool
}

func newFakeClock() *fakeClock { return &fakeClock{now: time.Unix(0, 0)} }

func (c *fakeClock) NewTimer(duration time.Duration) Timer {
	c.mu.Lock()
	defer c.mu.Unlock()
	timer := &fakeTimer{clock: c, channel: make(chan time.Time, 1), deadline: c.now.Add(duration), active: true}
	c.timers = append(c.timers, timer)
	return timer
}

func (c *fakeClock) Advance(duration time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(duration)
	now := c.now
	var due []*fakeTimer
	for _, timer := range c.timers {
		if timer.active && !timer.deadline.After(now) {
			timer.active = false
			due = append(due, timer)
		}
	}
	c.mu.Unlock()
	for _, timer := range due {
		timer.channel <- now
	}
}

func (c *fakeClock) timerCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.timers)
}

func (t *fakeTimer) C() <-chan time.Time { return t.channel }

func (t *fakeTimer) Stop() bool {
	t.clock.mu.Lock()
	defer t.clock.mu.Unlock()
	wasActive := t.active
	t.active = false
	return wasActive
}

func (t *fakeTimer) Reset(duration time.Duration) bool {
	t.clock.mu.Lock()
	defer t.clock.mu.Unlock()
	wasActive := t.active
	t.deadline = t.clock.now.Add(duration)
	t.active = true
	return wasActive
}

func TestDebouncerConsolidatesBurstWithControlledClock(t *testing.T) {
	clock := newFakeClock()
	debouncer := NewDebouncer(150*time.Millisecond, 500*time.Millisecond)
	debouncer.clock = clock
	events := make(chan struct{}, 8)
	output := make(chan struct{}, 8)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go debouncer.Run(ctx, events, output)

	events <- struct{}{}
	events <- struct{}{}
	events <- struct{}{}
	waitForTimerCount(t, clock, 2)
	clock.Advance(149 * time.Millisecond)
	assertNoDebounce(t, output)
	clock.Advance(time.Millisecond)
	assertDebounce(t, output)
	assertNoDebounce(t, output)
}

func TestContinuousBurstCannotStarvePastMaximum(t *testing.T) {
	clock := newFakeClock()
	debouncer := NewDebouncer(150*time.Millisecond, 500*time.Millisecond)
	debouncer.clock = clock
	events := make(chan struct{}, 8)
	output := make(chan struct{}, 8)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go debouncer.Run(ctx, events, output)

	events <- struct{}{}
	waitForTimerCount(t, clock, 2)
	for range 4 {
		clock.Advance(100 * time.Millisecond)
		events <- struct{}{}
		time.Sleep(time.Millisecond)
		assertNoDebounce(t, output)
	}
	clock.Advance(100 * time.Millisecond)
	assertDebounce(t, output)
}

func waitForTimerCount(t *testing.T, clock *fakeClock, count int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for clock.timerCount() < count && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if clock.timerCount() < count {
		t.Fatalf("timer count = %d, want at least %d", clock.timerCount(), count)
	}
}

func assertDebounce(t *testing.T, output <-chan struct{}) {
	t.Helper()
	select {
	case <-output:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for debounced output")
	}
}

func assertNoDebounce(t *testing.T, output <-chan struct{}) {
	t.Helper()
	select {
	case <-output:
		t.Fatal("unexpected debounced output")
	case <-time.After(5 * time.Millisecond):
	}
}
