// SPDX-FileCopyrightText: 2022 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

package karajo

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	liberrors "github.com/shuLhan/share/lib/errors"
	libhttp "github.com/shuLhan/share/lib/http"
	"github.com/shuLhan/share/lib/mlog"
	libhtml "github.com/shuLhan/share/lib/net/html"
)

// List of errors.
var (
	ErrJobEmptyCommandsOrCall = liberrors.E{
		Code:    http.StatusBadRequest,
		Name:    `ERR_JOB_EMPTY_COMMANDS_OR_CALL`,
		Message: "empty commands or call handle",
	}
	ErrJobForbidden = liberrors.E{
		Code:    http.StatusForbidden,
		Name:    `ERR_JOB_FORBIDDEN`,
		Message: "forbidden",
	}
	ErrJobInvalidSecret = liberrors.E{
		Code:    http.StatusBadRequest,
		Name:    `ERR_JOB_INVALID_SECRET`,
		Message: "invalid or empty secret",
	}
)

const (
	defJobLogRetention = 5

	jobEnvCounter   = `KARAJO_JOB_COUNTER`
	jobEnvPath      = `PATH`
	jobEnvPathValue = `/usr/local/sbin:/usr/local/bin:/usr/bin:/usr/bin/site_perl:/usr/bin/vendor_perl:/usr/bin/core_perl`
)

// JobHttpHandler define a function signature for running a Job using HTTP
// request.
// The log parameter should be used to log all output and error.
// The epr parameter contains HTTP request, body, and response writer.
type JobHttpHandler func(log io.Writer, epr *libhttp.EndpointRequest) error

// Job is a job that can be triggered manually by sending HTTP POST request
// or automatically by timer (per interval).
//
// For job triggered by HTTP request, the Path and Secret must be set.
// For job triggered by timer, the Interval must be positive duration, equal
// or greater than 1 minute.
//
// Each Job contains a working directory, and a callback or list of commands
// to be executed.
type Job struct {
	// Shared Environment.
	env *Environment `json:"-"`

	// Cache of log sorted by its counter.
	Logs []*JobLog

	// Call define a function or method to be called, as an
	// alternative to Commands.
	// This field is optional, it is only used if Job created through
	// code.
	Call JobHttpHandler `json:"-" ini:"-"`

	// HTTP path where Karajo will listen for request.
	// The Path is automatically prefixed with "/karajo/job", it is not
	// static.
	// For example, if it set to "/my", then the actual path would be
	// "/karajo/job/my".
	// This field is required and unique between Job.
	Path string `ini:"::path"`

	// HeaderSign define the HTTP header where the signature is read.
	// Default to "x-karajo-sign" if its empty.
	HeaderSign string `ini:"::header_sign"`

	// Secret define a string to check signature of request.
	// Each request sign the body with HMAC + SHA-256 using this secret.
	// The signature then sent in HTTP header "X-Karajo-Sign" as hex.
	// This field is required if Path is not empty.
	Secret string `ini:"::secret" json:"-"`

	// dirWork define the directory on the system where all commands
	// will be executed.
	dirWork string
	dirLog  string

	// Commands list of command to be executed.
	Commands []string `ini:"::command"`

	JobBase

	LogRetention int `ini:"::log_retention"`
	lastCounter  int64

	sync.Mutex
}

// finish mark the job as finished with status.
func (job *Job) finish(hlog *JobLog, status string) {
	var (
		err error
	)

	if hlog != nil {
		hlog.setStatus(status)
		err = hlog.flush()
		if err != nil {
			mlog.Errf(`job: %s: %s`, job.ID, err)
		}
	}

	job.Lock()
	job.NumRunning--
	job.LastRun = TimeNow().UTC().Round(time.Second)
	job.Status = status
	if job.Interval > 0 {
		job.NextRun = job.LastRun.Add(job.Interval)
	}

	select {
	case job.finished <- true:
	default:
	}
	job.Unlock()

	mlog.Outf(`job: %s: %s`, job.ID, status)
}

func (job *Job) generateCmdEnvs() (env []string) {
	env = append(env, fmt.Sprintf(`%s=%d`, jobEnvCounter, job.lastCounter))
	env = append(env, fmt.Sprintf(`%s=%s`, jobEnvPath, jobEnvPathValue))
	return env
}

// init initialize the Job.
//
// For Job that need to be triggered by HTTP request the Path and Secret
// _must_ not be empty, otherwise it will return an error
// ErrJobInvalidSecret.
//
// It will return an error ErrJobEmptyCommandsOrCall if one of the Call or
// Commands is not set.
func (job *Job) init(env *Environment, name string) (err error) {
	job.JobBase.init()

	job.Path = strings.TrimSpace(job.Path)
	job.Secret = strings.TrimSpace(job.Secret)
	if len(job.Path) != 0 && len(job.Secret) == 0 {
		return &ErrJobInvalidSecret
	}

	if len(job.Commands) == 0 && job.Call == nil {
		return &ErrJobEmptyCommandsOrCall
	}

	job.env = env
	job.Name = name
	job.ID = libhtml.NormalizeForID(name)
	if job.LogRetention <= 0 {
		job.LogRetention = defJobLogRetention
	}

	err = job.initDirsState(env)
	if err != nil {
		return err
	}

	err = job.initLogs()
	if err != nil {
		return err
	}

	job.initTimer()

	if len(job.HeaderSign) == 0 {
		job.HeaderSign = HeaderNameXKarajoSign
	}

	return nil
}

func (job *Job) initDirsState(env *Environment) (err error) {
	job.dirWork = filepath.Join(env.dirLibJob, job.ID)
	err = os.MkdirAll(job.dirWork, 0700)
	if err != nil {
		return err
	}

	job.dirLog = filepath.Join(env.dirLogJob, job.ID)
	err = os.MkdirAll(job.dirLog, 0700)
	if err != nil {
		return err
	}

	return nil
}

// initLogs load the job logs state, counter and status.
func (job *Job) initLogs() (err error) {
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

		if hlog.Counter > job.lastCounter {
			job.lastCounter = hlog.Counter
			job.Status = hlog.Status
		}

		fiModTime = fi.ModTime()
		if job.LastRun.IsZero() {
			job.LastRun = fiModTime
		} else if fiModTime.After(job.LastRun) {
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

// initTimer init fields that required to run Job with interval.
func (job *Job) initTimer() {
	if job.Interval <= 0 {
		return
	}
	if job.Interval < time.Minute {
		job.Interval = time.Minute
	}
}

func (job *Job) logsPrune() {
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

// handleHttp handle trigger to run the Job from HTTP request.
//
// Once the signature is verified it will response immediately and run the
// actual process in the new goroutine.
func (job *Job) handleHttp(epr *libhttp.EndpointRequest) (resbody []byte, err error) {
	var (
		res      libhttp.EndpointResponse
		zeroTime time.Time
		expSign  string
		gotSign  string
	)

	job.Lock()
	job.Status = JobStatusStarted
	job.LastRun = zeroTime
	job.Unlock()

	// Authenticated request by checking the request body.
	gotSign = epr.HttpRequest.Header.Get(job.HeaderSign)
	if len(gotSign) == 0 {
		gotSign = epr.HttpRequest.Header.Get(HeaderNameXKarajoSign)
		if len(gotSign) == 0 {
			job.finish(nil, JobStatusFailed)
			return nil, &ErrJobForbidden
		}
	}

	gotSign = strings.TrimPrefix(gotSign, "sha256=")

	expSign = Sign(epr.RequestBody, []byte(job.Secret))
	if expSign != gotSign {
		mlog.Outf(`job: %s: expecting signature %s got %s`, job.ID, expSign, gotSign)
		job.finish(nil, JobStatusFailed)
		return nil, &ErrJobForbidden
	}

	go job.execute(epr)

	res.Code = http.StatusOK
	res.Message = "OK"
	res.Data = job

	job.Lock()
	resbody, err = json.Marshal(&res)
	job.Unlock()

	return resbody, err
}

// Start the Job timer only if its Interval is non-zero.
func (job *Job) Start() {
	if job.Interval <= 0 {
		return
	}

	var (
		now          time.Time
		nextInterval time.Duration
		timer        *time.Timer
		ever         bool
	)

	for {
		job.Lock()
		now = TimeNow().UTC().Round(time.Second)
		nextInterval = job.computeNextInterval(now)
		job.NextRun = now.Add(nextInterval)
		job.Unlock()

		mlog.Outf(`job: %s: next running in %s ...`, job.ID, nextInterval)

		timer = time.NewTimer(nextInterval)
		ever = true
		for ever {
			select {
			case <-timer.C:
				job.execute(nil)
				// The execute will trigger the finished
				// channel.

			case <-job.finished:
				timer.Stop()
				ever = false

			case <-job.stopped:
				timer.Stop()
				return
			}
		}
	}
}

// execute the job Call or commands.
func (job *Job) execute(epr *libhttp.EndpointRequest) {
	var (
		hlog    *JobLog
		execCmd exec.Cmd
		now     time.Time
		cmd     string
		err     error
		x       int
	)

	job.env.jobq <- struct{}{}
	mlog.Outf("job: %s: started ...", job.ID)
	defer func() {
		<-job.env.jobq
	}()

	job.Lock()
	job.NumRunning++
	job.lastCounter++
	hlog = newJobLog(job.ID, job.dirLog, job.lastCounter)

	job.Logs = append(job.Logs, hlog)
	job.logsPrune()
	job.Unlock()

	// Call the job.
	if job.Call != nil {
		err = job.Call(hlog, epr)
		if err != nil {
			_, _ = hlog.Write([]byte(err.Error()))
			job.finish(hlog, JobStatusFailed)
		} else {
			job.finish(hlog, JobStatusSuccess)
		}
		return
	}

	// Run commands.
	for x, cmd = range job.Commands {
		now = TimeNow().UTC()
		fmt.Fprintf(hlog, "\n%s === Execute %2d: %s\n", now.Format(defTimeLayout), x, cmd)

		execCmd = exec.Cmd{
			Path:   "/bin/sh",
			Dir:    job.dirWork,
			Args:   []string{"/bin/sh", "-c", cmd},
			Env:    job.generateCmdEnvs(),
			Stdout: hlog,
			Stderr: hlog,
		}

		err = execCmd.Run()
		if err != nil {
			_, _ = hlog.Write([]byte(err.Error()))
			job.finish(hlog, JobStatusFailed)
			return
		}
	}

	job.finish(hlog, JobStatusSuccess)
}

// Stop the Job timer execution.
func (job *Job) Stop() {
	mlog.Outf(`job: %s: stopping ...`, job.ID)

	select {
	case job.stopped <- true:
	default:
	}
}
