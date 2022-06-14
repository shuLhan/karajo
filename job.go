// SPDX-FileCopyrightText: 2021 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

package karajo

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/shuLhan/share/lib/clise"
	libhttp "github.com/shuLhan/share/lib/http"
	"github.com/shuLhan/share/lib/mlog"
)

// List of job status.
const (
	JobStatusStarted = "started"
	JobStatusSuccess = "success"
	JobStatusFailed  = "failed"
	JobStatusPaused  = "paused"
)

// DefaultMaxRequests define maximum number of requests that can be
// executed simultaneously.
// This is to prevent the karajo server consume resources on the local
// server and on the remote server.
const DefaultMaxRequests = 1

const (
	defJobInterval = 30 * time.Second
	defJobLogsSize = 20
	defTimeLayout  = "2006-01-02 15:04:05 MST"
)

// Job is the worker that will trigger HTTP GET request to the remote job
// periodically and save the response status to logs and the last execution
// time for future run.
type Job struct {
	// The next time the job will running, in UTC.
	NextRun time.Time

	// The last time the job is running, in UTC.
	LastRun time.Time

	headers http.Header

	done chan bool

	logs *clise.Clise // logs contains 100 last jobs output.
	mlog *mlog.MultiLogger
	flog *os.File

	// httpc define the HTTP client that will execute the http_url.
	httpc *libhttp.Client

	// ID of the job. It must be unique or the last job will replace the
	// previous job with the same ID.
	// If ID is empty, it will generated from Name by replacing
	// non-alphanumeric character with '-'.
	ID string

	// Name of job for readibility.
	Name        string `ini:"::name"`
	Description string `ini:"::description"`

	//
	// The HTTP URL where the job will be executed.
	// This field is required.
	//
	HttpUrl    string `ini:"::http_url"`
	baseUri    string
	requestUri string

	// The last status of the job.
	LastStatus string

	// Optional HTTP headers for HttpUrl, in the format of "K: V".
	HttpHeaders []string `ini:"::http_header"`

	// HttpTimeout custom HTTP timeout for this job.  If its zero, it will
	// set from the Environment.HttpTimeout.
	// To make job run without timeout, set the value to negative.
	HttpTimeout time.Duration

	//
	// Interval duration when job will be repeatedly executed.
	// This field is required, if not set or invalid it will set to 10
	// minutes.
	//
	// If one have job that need to run every less than 10 minutes, it
	// should be run on single program.
	//
	Interval time.Duration `ini:"::interval"`

	// MaxRequests maximum number of requests executed by karajo.
	// If zero, it will set to DefaultMaxRequests.
	MaxRequests int8 `ini:"::max_requests"`

	// NumRequests record the current number of requests executed.
	NumRequests int8

	locker sync.Mutex

	//
	// HttpInsecure can be set to true if the http_url is HTTPS with
	// unknown certificate authority.
	//
	HttpInsecure bool `ini:"::http_insecure"`

	// IsPausing if its true, the job execution will be skipped.
	IsPausing bool
}

func (job *Job) Start() (err error) {
	now := time.Now().UTC().Round(time.Second)
	job.NumRequests = 0

	job.mlog.Outf("starting job: %+v\n", job)

	firstTimer := job.computeFirstTimer(now)
	job.NextRun = now.Add(firstTimer)
	job.mlog.Outf("running the first job in %s ...\n", firstTimer)

	t := time.NewTimer(firstTimer)
	ever := true
	for ever {
		select {
		case <-t.C:
			job.execute()
			t.Stop()
			ever = false
		case <-job.done:
			return nil
		}
	}

	job.NextRun = job.LastRun.Add(job.Interval)
	job.mlog.Outf("running the next job at %s ...\n", job.NextRun.Format(defTimeLayout))

	tick := time.NewTicker(job.Interval)
	for {
		select {
		case <-tick.C:
			job.execute()
			job.NextRun = job.LastRun.Add(job.Interval)
			job.mlog.Outf("running the next job at %s\n", job.NextRun.Format(defTimeLayout))

		case <-job.done:
			return nil
		}
	}

	return nil
}

// Stop the job.
func (job *Job) Stop() {
	job.mlog.Outf("stopping job ...\n")
	job.done <- true

	job.mlog.Flush()
	if job.flog != nil {
		err := job.flog.Close()
		if err != nil {
			mlog.Errf("Stop %s: %s", job.ID, err)
		}
	}
}

// init initialize the job, compute the last run and the next run.
func (job *Job) init(env *Environment) (err error) {
	if len(job.ID) == 0 {
		job.ID = generateID(job.Name)
	}

	err = job.initLogger(env)
	if err != nil {
		return err
	}

	err = job.initHttpUrl(env.ListenAddress)
	if err != nil {
		return err
	}

	err = job.initHttpHeaders()
	if err != nil {
		return err
	}

	httpClientOpts := &libhttp.ClientOptions{
		ServerUrl:     job.baseUri,
		Headers:       job.headers,
		AllowInsecure: job.HttpInsecure,
	}
	job.httpc = libhttp.NewClient(httpClientOpts)

	if job.HttpTimeout == 0 {
		job.HttpTimeout = env.HttpTimeout
	} else if job.HttpTimeout < 0 {
		// Negative value means 0 on net/http.Client.
		job.HttpTimeout = 0
	}
	job.httpc.Client.Timeout = job.HttpTimeout

	job.logs = clise.New(defJobLogsSize)

	if job.Interval <= defJobInterval {
		job.Interval = defJobInterval
	}
	if job.MaxRequests == 0 {
		job.MaxRequests = DefaultMaxRequests
	}

	job.done = make(chan bool)

	return nil
}

// initLogger initialize the job logs location.
// By default all logs are written to os.Stdout and os.Stderr.
//
// If the Dir field on LogOptions is set, then all logs will written to file
// named "LogOptions.FilenamePrefix + job.ID" in those directory.
func (job *Job) initLogger(env *Environment) (err error) {
	job.mlog = mlog.NewMultiLogger(defTimeLayout, job.ID+":", nil, nil)
	job.mlog.RegisterErrorWriter(mlog.NewNamedWriter("stderr", os.Stderr))
	job.mlog.RegisterOutputWriter(mlog.NewNamedWriter("stdout", os.Stdout))

	if len(env.DirLogs) == 0 {
		return nil
	}

	logFile := env.name + "-" + job.ID
	logPath := filepath.Join(env.DirLogs, logFile)
	job.flog, err = os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("initLogger %s: %w", logPath, err)
	}

	nw := mlog.NewNamedWriter(logFile, job.flog)
	job.mlog.RegisterErrorWriter(nw)
	job.mlog.RegisterOutputWriter(nw)

	return nil
}

func (job *Job) initHttpUrl(serverAddress string) (err error) {
	if job.HttpUrl[0] == '/' {
		job.baseUri = fmt.Sprintf("http://%s", serverAddress)
		job.requestUri = job.HttpUrl
	} else {
		httpUrl, err := url.Parse(job.HttpUrl)
		if err != nil {
			return fmt.Errorf("%s: invalid http_url %q: %w",
				job.ID, job.HttpUrl, err)
		}

		port := httpUrl.Port()
		if len(port) == 0 {
			if httpUrl.Scheme == "https" {
				port = "443"
			} else {
				port = "80"
			}
		}

		job.baseUri = fmt.Sprintf("%s://%s:%s", httpUrl.Scheme,
			httpUrl.Hostname(), port)
		job.requestUri = httpUrl.RequestURI()
	}

	return nil
}

func (job *Job) initHttpHeaders() (err error) {
	if len(job.HttpHeaders) > 0 {
		job.headers = make(http.Header, len(job.HttpHeaders))
	}

	for _, h := range job.HttpHeaders {
		kv := strings.SplitN(h, ":", 2)
		if len(kv) != 2 {
			return fmt.Errorf("%s: invalid header %q", job.ID, h)
		}

		job.headers.Set(strings.TrimSpace(kv[0]), strings.TrimSpace(kv[1]))
	}
	return nil
}

func (job *Job) increment() (ok bool) {
	job.locker.Lock()
	if job.NumRequests+1 <= job.MaxRequests {
		job.NumRequests++
		ok = true
	}
	job.locker.Unlock()
	return ok
}

func (job *Job) decrement() {
	job.locker.Lock()
	job.NumRequests--
	job.locker.Unlock()
}

func (job *Job) execute() {
	now := time.Now().UTC().Round(time.Second)
	logTime := now.Format(defTimeLayout)

	if job.isPausing() {
		msg := "paused\n"
		job.mlog.Outf(msg)
		job.logs.Push(fmt.Sprintf("%s %s: %s", logTime, job.ID, msg))
		job.LastStatus = JobStatusPaused
		job.LastRun = now
		return
	}

	if !job.increment() {
		log := fmt.Sprintf("!!! maximum requests %d has been reached", job.MaxRequests)
		job.mlog.Errf(log)
		job.logs.Push(fmt.Sprintf("%s %s: %s", logTime, job.ID, log))
		return
	}
	defer job.decrement()

	httpRes, resBody, err := job.httpc.Get(job.requestUri, nil, nil)
	if err != nil {
		log := fmt.Sprintf("!!! %s", err)
		job.mlog.Errf(log)
		job.logs.Push(fmt.Sprintf("%s %s: %s", logTime, job.ID, log))
		job.LastStatus = JobStatusFailed
		job.LastRun = now
		return
	}

	if httpRes.StatusCode != http.StatusOK {
		log := fmt.Sprintf("!!! %s: %s", httpRes.Status, resBody)
		job.mlog.Errf(log)
		job.logs.Push(fmt.Sprintf("%s %s: %s", logTime, job.ID, log))
		job.LastStatus = JobStatusFailed
		job.LastRun = now
		return
	}

	log := fmt.Sprintf(">>> %s\n", resBody)
	job.mlog.Outf(log)
	job.logs.Push(fmt.Sprintf("%s %s: %s", logTime, job.ID, log))
	job.LastStatus = JobStatusSuccess
	job.LastRun = now
}

// computeFirstTimer compute the duration when the job will be running based
// on last time run and interval.
//
// If the `(last_run + interval) < now` then it will return 0; otherwise it will
// return `(last_run + interval) - now`
func (job *Job) computeFirstTimer(now time.Time) time.Duration {
	lastInterval := job.LastRun.Add(job.Interval)
	if lastInterval.Before(now) {
		return 0
	}
	return lastInterval.Sub(now)
}

func (job *Job) pause() {
	job.mlog.Outf("pausing...\n")
	job.locker.Lock()
	job.IsPausing = true
	job.locker.Unlock()
}

func (job *Job) resume() {
	job.mlog.Outf("resuming...\n")
	job.locker.Lock()
	job.IsPausing = false
	job.LastStatus = JobStatusStarted
	job.locker.Unlock()
}

func (job *Job) isPausing() (b bool) {
	job.locker.Lock()
	b = job.IsPausing
	job.locker.Unlock()
	return b
}
