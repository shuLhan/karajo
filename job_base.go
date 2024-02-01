// SPDX-FileCopyrightText: 2023 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

package karajo

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/shuLhan/share/lib/mlog"
	libhtml "github.com/shuLhan/share/lib/net/html"
	libtime "github.com/shuLhan/share/lib/time"
)

// List of [JobBase.Status].
// The job status have the following cycle,
//
//	started --> running -+-> success --+
//	                     |             +--> paused --> started
//		             +-> failed  --+
const (
	JobStatusFailed  = `failed`
	JobStatusPaused  = `paused`
	JobStatusRunning = `running`
	JobStatusStarted = `started`
	JobStatusSuccess = `success`
)

// JobBase define the base fields and commons methods for all job types.
//
// The base configuration in INI format,
//
//	[job "name"]
//	description =
//	schedule =
//	interval =
//	log_retention =
//	notif_on_success =
//	notif_on_failed =
type JobBase struct {
	// The last time the job is finished running, in UTC.
	LastRun time.Time `ini:"-" json:"last_run,omitempty"`

	// The next time the job will running, in UTC.
	NextRun time.Time `ini:"-" json:"next_run,omitempty"`

	scheduler *libtime.Scheduler

	// logq is publish-only channel passed by Karajo instance to
	// communicate job log for notification.
	logq chan<- *JobLog

	// ID of the job.
	// It must be unique, otherwise when jobs loaded, the last job will
	// replace the previous job with the same ID.
	// If ID is empty, it will generated from Name by replacing
	// non-alphanumeric character with '-'.
	ID string `ini:"-" json:"id"`

	// Name of job for human.
	Name string `ini:"-" json:"name"`

	// Description of the Job.
	// It could contains simple HTML tags.
	Description string `ini:"::description" json:"description,omitempty"`

	// Status of the job on last execution.
	Status string `ini:"-" json:"status,omitempty"`

	// Schedule a timer that run periodically based on calendar or day
	// time.
	// A schedule is divided into monthly, weekly, daily, hourly, and
	// minutely.
	// See [time.Scheduler] for format of schedule.
	//
	// If both Schedule and Interval set, only Schedule will be processed.
	//
	// [time.Scheduler]: https://pkg.go.dev/github.com/shuLhan/share/lib/time#Scheduler
	Schedule string `ini:"::schedule" json:"schedule,omitempty"`

	// dirWork define the directory on the system where all commands
	// will be executed.
	dirWork string

	dirLog string

	// NotifOnSuccess define list of notification where the job's log will
	// be send when job execution finish successfully.
	NotifOnSuccess []string `ini:"::notif_on_success" json:"notif_on_success,omitempty"`

	// NotifOnFailed define list of notification where the job's log will
	// be send when job execution failed.
	NotifOnFailed []string `ini:"::notif_on_failed" json:"notif_on_failed,omitempty"`

	kind jobKind

	// Logs contains cache of log sorted by its counter.
	Logs []*JobLog `json:"logs,omitempty"`

	// Interval duration when job will be repeatedly executed.
	// This field is optional, the minimum value is one minute.
	//
	// If both Schedule and Interval set, only Schedule will be processed.
	Interval time.Duration `ini:"::interval" json:"interval,omitempty"`

	counter int64

	// LogRetention define the maximum number of logs to keep in storage.
	// This field is optional, default to 5.
	LogRetention int `ini:"::log_retention" json:"log_retention,omitempty"`

	sync.Mutex
}

// init initialize the job ID, log retention, directories, logs, and timer.
func (job *JobBase) init(env *Env, name string) (err error) {
	var logp = `init`

	job.Name = name
	job.ID = libhtml.NormalizeForID(name)
	job.Status = JobStatusStarted

	if job.LogRetention <= 0 {
		job.LogRetention = defJobLogRetention
	}

	err = job.initDirsState(env)
	if err != nil {
		return fmt.Errorf(`%s: %w`, logp, err)
	}

	err = job.initLogs()
	if err != nil {
		return fmt.Errorf(`%s: %w`, logp, err)
	}

	err = job.initTimer()
	if err != nil {
		return fmt.Errorf(`%s: %w`, logp, err)
	}
	return nil
}

// initDirsState initialize the job working and log directories.
//
// For job with type exec, the working directory should be at
// "$BASE/var/lib/karajo/job/$JOB_ID" and the log should be at
// "$BASE/var/log/karajo/job/$JOB_ID".
//
// For job with type http, the working directory should be at
// "$BASE/var/lib/karajo/job_http/$JOB_ID" and the log should be at
// "$BASE/var/log/karajo/job_http/$JOB_ID".
func (job *JobBase) initDirsState(env *Env) (err error) {
	var logp = `initDirsState`

	switch job.kind {
	case jobKindExec:
		job.dirWork = filepath.Join(env.dirLibJob, job.ID)
		err = os.MkdirAll(job.dirWork, 0700)
		if err != nil {
			return fmt.Errorf(`%s: %w`, logp, err)
		}

		job.dirLog = filepath.Join(env.dirLogJob, job.ID)
		err = os.MkdirAll(job.dirLog, 0700)
		if err != nil {
			return fmt.Errorf(`%s: %w`, logp, err)
		}

		return nil

	case jobKindHTTP:
		job.dirWork = filepath.Join(env.dirLibJobHTTP, job.ID)
		err = os.MkdirAll(job.dirWork, 0700)
		if err != nil {
			return fmt.Errorf(`%s: %w`, logp, err)
		}

		job.dirLog = filepath.Join(env.dirLogJobHTTP, job.ID)

		// Remove previous log file.
		_ = os.Remove(job.dirLog)

		err = os.MkdirAll(job.dirLog, 0700)
		if err != nil {
			return fmt.Errorf(`%s: %w`, logp, err)
		}
	}
	return nil
}

// initLogs load the job logs state, counter, and status.
//
// For each file in job's log directory, parse the log file name in the form
// of "$JOB_ID.$COUNTER.$STATUS" to get its counter and status.
//
// The logs then stored in ascending order by its counter.
func (job *JobBase) initLogs() (err error) {
	var (
		dir       *os.File
		hlog      *JobLog
		fi        os.FileInfo
		fiModTime time.Time
		fis       []os.FileInfo
	)

	dir, err = os.Open(job.dirLog)
	if err != nil {
		return err
	}
	fis, err = dir.Readdir(0)
	if err != nil {
		return err
	}

	for _, fi = range fis {
		hlog = parseJobLogName(job.dirLog, fi.Name())
		if hlog == nil {
			// Skip log with invalid file name.
			continue
		}

		job.Logs = append(job.Logs, hlog)

		if hlog.Counter > job.counter {
			job.counter = hlog.Counter
			job.Status = hlog.Status
		}

		fiModTime = fi.ModTime()
		if fiModTime.After(job.LastRun) {
			job.LastRun = fiModTime
		}
	}

	job.LastRun = job.LastRun.UTC().Round(time.Second)

	sort.Slice(job.Logs, func(x, y int) bool {
		return job.Logs[x].Counter < job.Logs[y].Counter
	})

	job.logsPrune()

	return nil
}

// initTimer init fields that required to run Job with Interval or Schedule.
func (job *JobBase) initTimer() (err error) {
	var logp = `initTimer`

	if len(job.Schedule) != 0 {
		job.scheduler, err = libtime.NewScheduler(job.Schedule)
		if err != nil {
			return fmt.Errorf(`%s: %w`, logp, err)
		}

		// Since only Schedule or Interval can be run, unset the
		// Interval here.
		job.Interval = 0
		job.NextRun = job.scheduler.Next()
		return
	}
	if job.Interval > 0 {
		if job.Interval < time.Minute {
			job.Interval = time.Minute
		}

		var (
			now          = timeNow()
			nextInterval = job.computeNextInterval(now)
		)
		job.NextRun = now.Add(nextInterval)
	}
	return nil
}

// getLog get the JobLog by its counter.
func (job *JobBase) getLog(counter int64) (jlog *JobLog) {
	job.Lock()
	for _, jlog = range job.Logs {
		if jlog.Counter == counter {
			job.Unlock()
			return jlog
		}
	}
	job.Unlock()
	return nil
}

// logsPrune remove log files based on number of logs retention policy.
// This function assume that Logs has been sorted in ascending order.
//
// For example, if total logs is 10 and log retention is 5, the first five log
// items will be pruned.
func (job *JobBase) logsPrune() {
	var (
		hlog     *JobLog
		totalLog int
		indexMin int
	)

	totalLog = len(job.Logs)
	if totalLog > job.LogRetention {
		// Delete old logs.
		indexMin = totalLog - job.LogRetention
		for _, hlog = range job.Logs[:indexMin] {
			_ = os.Remove(hlog.path)
		}
		job.Logs = job.Logs[indexMin:]
	}
}

// newLog create new JobLog.
func (job *JobBase) newLog() (jlog *JobLog) {
	job.Lock()
	defer job.Unlock()

	job.counter++

	jlog = &JobLog{
		jobKind: job.kind,
		JobID:   job.ID,
		Name:    fmt.Sprintf(`%s.%d`, job.ID, job.counter),
		Counter: job.counter,
	}

	jlog.path = filepath.Join(job.dirLog, jlog.Name)

	if job.Status == JobStatusPaused {
		jlog.Status = JobStatusPaused
	} else {
		job.Status = JobStatusRunning
		jlog.Status = JobStatusRunning
	}

	job.Logs = append(job.Logs, jlog)
	job.logsPrune()

	return jlog
}

// canStart check if the job can be started or return an error if its paused
// or reached maximum running.
func (job *JobBase) canStart() (err error) {
	job.Lock()
	if job.Status == JobStatusPaused {
		err = ErrJobPaused
	}
	job.Unlock()
	return err
}

// finish mark the job as finished.
// If job finish with error, it will set the status to failed; otherwise to
// success.
func (job *JobBase) finish(jlog *JobLog, err error) {
	job.Lock()
	defer job.Unlock()

	if err != nil {
		var logv = fmt.Sprintf("!!! %s: %s: %s\n", job.kind, job.ID, err)
		jlog.Write([]byte(logv))
		mlog.Errf(logv)
		job.Status = JobStatusFailed
	} else {
		if jlog.Status != JobStatusPaused {
			job.Status = JobStatusSuccess
			fmt.Fprintf(jlog, "=== %s: %s: finished.\n", job.kind, job.ID)
		}
	}

	jlog.setStatus(job.Status)
	err = jlog.flush()
	if err != nil {
		mlog.Errf(`job: %s: %s`, job.ID, err)
	}

	job.LastRun = timeNow()
	if job.scheduler != nil {
		job.NextRun = job.scheduler.Next()
	} else if job.Interval > 0 {
		job.NextRun = job.LastRun.Add(job.Interval)
	}

	if jlog.Status == JobStatusPaused {
		return
	}

	if job.kind == jobKindExec {
		switch jlog.Status {
		case JobStatusSuccess:
			jlog.listNotif = append(jlog.listNotif, job.NotifOnSuccess...)

		case JobStatusFailed:
			jlog.listNotif = append(jlog.listNotif, job.NotifOnFailed...)
		}
	}

	select {
	case job.logq <- jlog:
	default:
	}
}

// computeNextInterval compute the duration when the job will be running based
// on last time run and interval.
//
// If the `(last_run + interval) < now` then it will return 0; otherwise it will
// return `(last_run + interval) - now`
func (job *JobBase) computeNextInterval(now time.Time) time.Duration {
	var lastTime = job.LastRun.Add(job.Interval)
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
