// SPDX-FileCopyrightText: 2023 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

package karajo

import (
	"bytes"
	"sync"
	"time"

	"github.com/shuLhan/share/lib/mlog"
	libtime "github.com/shuLhan/share/lib/time"
)

// List of job status.
const (
	JobStatusRunning = `running`
	JobStatusStarted = `started`
	JobStatusSuccess = `success`
	JobStatusFailed  = `failed`
	JobStatusPaused  = `paused`
)

// JobBase define the base fields and commons methods for all Job types.
type JobBase struct {
	scheduler *libtime.Scheduler

	finishq chan struct{}

	// The last time the job is finished running, in UTC.
	LastRun time.Time `ini:"-" json:"last_run,omitempty"`

	// The next time the job will running, in UTC.
	NextRun time.Time `ini:"-" json:"next_run,omitempty"`

	// ID of the job. It must be unique or the last job will replace the
	// previous job with the same ID.
	// If ID is empty, it will generated from Name by replacing
	// non-alphanumeric character with '-'.
	ID string `ini:"-" json:"id"`

	// Name of job for readibility.
	Name string `ini:"-" json:"name"`

	// The description of the Job.
	// It could be plain text or simple HTML.
	Description string `ini:"::description" json:"description,omitempty"`

	// The last status of the job.
	Status string `ini:"-" json:"status,omitempty"`

	// Schedule a timer that run periodically based on calendar or day
	// time.
	// A schedule is divided into monthly, weekly, daily, hourly, and
	// minutely.
	// See [time.Scheduler] for format of schedule.
	//
	// If both Schedule and Interval set, only Schedule will be processed.
	//
	// [time.Scheduler]: // https://pkg.go.dev/github.com/shuLhan/share/lib/time#Scheduler
	Schedule string `ini:"::schedule" json:"schedule,omitempty"`

	// Interval duration when job will be repeatedly executed.
	// This field is optional, the minimum value is 1 minute.
	//
	// If both Schedule and Interval set, only Schedule will be processed.
	Interval time.Duration `ini:"::interval" json:"interval,omitempty"`

	sync.Mutex
}

func (job *JobBase) init() {
	job.finishq = make(chan struct{}, 1)
}

// canStart check if the job can be started or return an error if its paused
// or reached maximum running.
func (job *JobBase) canStart() (err error) {
	job.Lock()
	defer job.Unlock()

	if job.Status == JobStatusPaused {
		return ErrJobPaused
	}
	return nil
}

// start check if the job can run, the job is not paused and has not reach
// maximum run.
// If its can run, the status changes to `started`.
//
// If the job is paused, the LastRun will be set to current time and return
// ErrJobPaused.
func (job *JobBase) start() (err error) {
	err = job.canStart()
	if err != nil {
		return err
	}

	job.Lock()
	job.Status = JobStatusStarted
	job.Unlock()

	return nil
}

// finish mark the job as finished.
// If the err is not nil, it will set the status to failed; otherwise to
// success.
func (job *JobBase) finish(jlog *JobLog, err error) {
	job.Lock()
	defer job.Unlock()

	if err != nil {
		job.Status = JobStatusFailed
		if jlog != nil {
			_, _ = jlog.Write([]byte(err.Error()))
		}
	} else {
		job.Status = JobStatusSuccess
	}

	if jlog != nil {
		jlog.setStatus(job.Status)
		err = jlog.flush()
		if err != nil {
			mlog.Errf(`job: %s: %s`, job.ID, err)
		}
	}

	job.LastRun = TimeNow().UTC().Round(time.Second)
	if job.scheduler != nil {
		job.NextRun = job.scheduler.Next()
	} else if job.Interval > 0 {
		job.NextRun = job.LastRun.Add(job.Interval)
	}
}

// computeNextInterval compute the duration when the job will be running based
// on last time run and interval.
//
// If the `(last_run + interval) < now` then it will return 0; otherwise it will
// return `(last_run + interval) - now`
func (job *JobBase) computeNextInterval(now time.Time) time.Duration {
	var lastTime time.Time = job.LastRun.Add(job.Interval)
	if lastTime.Before(now) {
		return 0
	}
	return lastTime.Sub(now).Round(time.Second)
}

// pause the job execution.
func (job *JobBase) pause() {
	job.Lock()
	job.Status = JobStatusPaused
	job.Unlock()
}

// resume the job execution.
func (job *JobBase) resume(status string) {
	job.Lock()
	job.Status = status
	job.Unlock()
}

// packState convert the Job state into text, each field from top to bottom
// separated by new line.
func (job *JobBase) packState() (text []byte, err error) {
	var (
		buf bytes.Buffer
		raw []byte
	)

	raw, err = job.LastRun.MarshalText()
	if err != nil {
		return nil, err
	}
	buf.Write(raw)
	buf.WriteByte('\n')
	buf.WriteString(job.Status)
	buf.WriteByte('\n')
	return buf.Bytes(), nil
}

// unpackState load the Job state from text.
func (job *JobBase) unpackState(text []byte) (err error) {
	var (
		fields [][]byte = bytes.Split(text, []byte("\n"))
	)
	if len(fields) == 0 {
		return nil
	}
	err = job.LastRun.UnmarshalText(fields[0])
	if err != nil {
		return err
	}
	if len(fields) == 1 {
		return nil
	}
	job.Status = string(fields[1])
	return nil
}
