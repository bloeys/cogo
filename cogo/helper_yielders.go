package cogo

import "time"

var _ Yielder = &Sleeper{}

type Sleeper struct {
	wakeupTime time.Time
}

func (s *Sleeper) Tick() bool {
	return !time.Now().Before(s.wakeupTime)
}

// NewSleeper returns a sleeper that is done after at least sleepDuration time has passed
func NewSleeper(sleepDuration time.Duration) *Sleeper {
	return &Sleeper{
		wakeupTime: time.Now().Add(sleepDuration),
	}
}
