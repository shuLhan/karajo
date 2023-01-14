// SPDX-FileCopyrightText: 2023 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

package karajo

import (
	"time"
)

// JobBase define the base fields and commons methods for all Job types.
type JobBase struct {
	finished chan bool
	stopped  chan bool

	// The last time the Hook is finished running, in UTC.
	LastRun time.Time `ini:"-"`

	// The next time the job will running, in UTC.
	NextRun time.Time `ini:"-"`

	// Interval duration when job will be repeatedly executed.
	// This field is optional, the minimum value is 1 minute.
	Interval time.Duration `ini:"::interval"`

	// NumRunning record the current number of job currently running.
	NumRunning int `ini:"-"`
}

func (job *JobBase) init() {
	job.finished = make(chan bool, 1)
	job.stopped = make(chan bool, 1)
}

// computeNextInterval compute the duration when the job will be running based
// on last time run and interval.
//
// If the `(last_run + interval) < now` then it will return 0; otherwise it will
// return `(last_run + interval) - now`
func (job *JobBase) computeNextInterval(now time.Time) time.Duration {
	var lastInterval time.Time = job.LastRun.Add(job.Interval)
	if lastInterval.Before(now) {
		return 0
	}
	return lastInterval.Sub(now)
}
