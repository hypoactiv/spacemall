package game

import "time"

type ScheduleFunc func() error

func init() {

}

// Call f() every Duration d. Stop if f returns non-nil error.
// TODO: This will probably drop ticks if we're running slow
func scheduleRegister(d time.Duration, f ScheduleFunc) {
	go func() {
		t := time.NewTicker(d)
		for range t.C {
			if f() != nil {
				break
			}
		}
	}()
}
