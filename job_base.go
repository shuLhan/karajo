// SPDX-FileCopyrightText: 2023 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

package karajo

import (
	"bytes"
	"sync"
	"time"

	"github.com/shuLhan/share/lib/mlog"
)

// List of job status.
const (
	JobStatusRunning = `running`
	JobStatusStarted = "started"
	JobStatusSuccess = "success"
	JobStatusFailed  = "failed"
	JobStatusPaused  = "paused"
)

// DefaultJobMaxRunning define maximum number of job that can be
// executed simultaneously.
// This is to prevent the karajo server consume resources on the local
// server and on the remote server.
const DefaultJobMaxRunning = 1

// JobBase define the base fields and commons methods for all Job types.
type JobBase struct {
	finished chan bool
	stopped  chan bool

	// The last time the job is finished running, in UTC.
	LastRun time.Time `ini:"-"`

	// The next time the job will running, in UTC.
	NextRun time.Time `ini:"-"`

	// ID of the job. It must be unique or the last job will replace the
	// previous job with the same ID.
	// If ID is empty, it will generated from Name by replacing
	// non-alphanumeric character with '-'.
	ID string `ini:"-"`

	// Name of job for readibility.
	Name string `ini:"-"`

	// The description of the Job.
	// It could be plain text or simple HTML.
	Description string `ini:"::description"`

	// The last status of the job.
	Status string

	// Interval duration when job will be repeatedly executed.
	// This field is optional, the minimum value is 1 minute.
	Interval time.Duration `ini:"::interval"`

	// MaxRunning maximum number of job running at the same time.
	// This field is optional default to DefaultJobMaxRunning.
	MaxRunning int `ini:"::max_running"`

	// NumRunning record the number of job currently running.
	NumRunning int `ini:"-"`

	sync.Mutex
}

func (job *JobBase) init() {
	job.finished = make(chan bool, 1)
	job.stopped = make(chan bool, 1)

	if job.MaxRunning == 0 {
		job.MaxRunning = DefaultJobMaxRunning
	}
}

// start check if the job can run, the job is not paused and has not reach
// maximum run.
// If its can run, the status changes to "started" and its NumRunning
// increased by one.
//
// If the job is paused, the LastRun will be set to current time and return
// ErrJobPaused.
// if the max running has reached it will return ErrJobMaxReached.
func (job *JobBase) start() (err error) {
	job.Lock()
	defer job.Unlock()

	if job.Status == JobStatusPaused {
		// Always set the LastRun to the current time, otherwise it
		// will run with 0s duration for interval based job.
		job.LastRun = TimeNow().UTC().Round(time.Second)
		return ErrJobPaused
	}
	if job.NumRunning+1 > job.MaxRunning {
		return ErrJobMaxReached
	}

	job.NumRunning++
	job.Status = JobStatusStarted

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

	job.NumRunning--
	job.LastRun = TimeNow().UTC().Round(time.Second)
	if job.Interval > 0 {
		job.NextRun = job.LastRun.Add(job.Interval)
	}

	select {
	case job.finished <- true:
	default:
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
