// SPDX-FileCopyrightText: 2021 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

package karajo

import (
	"bytes"
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
	libhtml "github.com/shuLhan/share/lib/net/html"
)

// DefaultMaxRequests define maximum number of requests that can be
// executed simultaneously.
// This is to prevent the karajo server consume resources on the local
// server and on the remote server.
const DefaultMaxRequests = 1

const (
	defJobInterval    = 30 * time.Second
	defJobLogSize     = 20
	defJobLogSizeLoad = 2048
	defTimeLayout     = "2006-01-02 15:04:05 MST"
)

// Job is the worker that will trigger HTTP GET request to the remote job
// periodically and save the response status to log and the last execution
// time for future run.
type Job struct {
	jobState

	// The next time the job will running, in UTC.
	NextRun time.Time

	headers http.Header

	done chan bool

	// Log contains the Job output.
	// Upon started it will load several kilobytes lines from previous
	// log.
	Log *clise.Clise

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

	// Path to the job log.
	pathLog string

	// Path to the last job state.
	pathState string

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

	sync.Mutex

	//
	// HttpInsecure can be set to true if the http_url is HTTPS with
	// unknown certificate authority.
	//
	HttpInsecure bool `ini:"::http_insecure"`
}

func (job *Job) Start() {
	var (
		now        = time.Now().UTC().Round(time.Second)
		firstTimer = job.computeFirstTimer(now)
		ever       = true

		t    *time.Timer
		tick *time.Ticker
	)

	job.NumRequests = 0

	job.mlog.Outf("starting job: %+v\n", job)

	job.NextRun = now.Add(firstTimer)
	job.mlog.Outf("running the first job in %s ...\n", firstTimer)

	t = time.NewTimer(firstTimer)
	for ever {
		select {
		case <-t.C:
			job.execute()
			t.Stop()
			ever = false
		case <-job.done:
			return
		}
	}

	job.NextRun = job.LastRun.Add(job.Interval)
	job.mlog.Outf("running the next job at %s ...\n", job.NextRun.Format(defTimeLayout))

	tick = time.NewTicker(job.Interval)
	for {
		select {
		case <-tick.C:
			job.execute()
			job.NextRun = job.LastRun.Add(job.Interval)
			job.mlog.Outf("running the next job at %s\n", job.NextRun.Format(defTimeLayout))

		case <-job.done:
			return
		}
	}
}

// Stop the job.
func (job *Job) Stop() {
	job.mlog.Outf("stopping job ...\n")
	job.done <- true

	job.mlog.Flush()
	var err error = job.flog.Close()
	if err != nil {
		mlog.Errf("Stop %s: %s", job.ID, err)
	}
}

// init initialize the job, compute the last run and the next run.
func (job *Job) init(env *Environment) (err error) {
	if len(job.ID) == 0 {
		job.ID = libhtml.NormalizeForID(job.Name)
	}

	job.pathLog = filepath.Join(env.dirLogJob, job.ID)
	err = job.initLogger(env)
	if err != nil {
		return err
	}

	job.pathState = filepath.Join(env.dirRunJob, job.ID)
	err = job.stateLoad()
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

	var httpClientOpts = &libhttp.ClientOptions{
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

	if job.Interval <= defJobInterval {
		job.Interval = defJobInterval
	}
	if job.MaxRequests == 0 {
		job.MaxRequests = DefaultMaxRequests
	}

	job.done = make(chan bool)

	return nil
}

// initLogger initialize the job log and its location.
// By default log are written to os.Stdout and os.Stderr;
// and then to file named job.ID in Environment.dirLogJob.
func (job *Job) initLogger(env *Environment) (err error) {
	var (
		logp    = "initLogger"
		lastLog = make([]byte, defJobLogSizeLoad)

		fi      os.FileInfo
		nw      mlog.NamedWriter
		logs    [][]byte
		logLine []byte
		readOff int64
	)

	job.Log = clise.New(defJobLogSize)

	job.mlog = mlog.NewMultiLogger(defTimeLayout, job.ID+":", nil, nil)
	job.mlog.RegisterErrorWriter(mlog.NewNamedWriter("stderr", os.Stderr))
	job.mlog.RegisterOutputWriter(mlog.NewNamedWriter("stdout", os.Stdout))

	job.flog, err = os.OpenFile(job.pathLog, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0600)
	if err != nil {
		return fmt.Errorf("%s %s: %w", logp, job.pathLog, err)
	}

	// Load the last logs.

	fi, err = job.flog.Stat()
	if err != nil {
		return fmt.Errorf("%s: %w", logp, err)
	}

	readOff = fi.Size() - defJobLogSizeLoad
	if readOff < 0 {
		readOff = 0
	}

	_, err = job.flog.ReadAt(lastLog, readOff)
	if err != nil {
		return fmt.Errorf("%s: %w", logp, err)
	}

	logs = bytes.Split(lastLog, []byte{'\n'})
	if len(logs) > 0 {
		// Skip the first line, since it may not a complete log line.
		for _, logLine = range logs[1:] {
			job.Log.Push(string(logLine))
		}
	}

	// Forward the log to file.

	nw = mlog.NewNamedWriter(job.ID, job.flog)

	job.mlog.RegisterErrorWriter(nw)
	job.mlog.RegisterOutputWriter(nw)

	return nil
}

func (job *Job) initHttpUrl(serverAddress string) (err error) {
	if job.HttpUrl[0] == '/' {
		job.baseUri = fmt.Sprintf("http://%s", serverAddress)
		job.requestUri = job.HttpUrl
		return nil
	}

	var (
		httpUrl *url.URL
		port    string
	)

	httpUrl, err = url.Parse(job.HttpUrl)
	if err != nil {
		return fmt.Errorf("%s: invalid http_url %q: %w", job.ID, job.HttpUrl, err)
	}

	port = httpUrl.Port()
	if len(port) == 0 {
		if httpUrl.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}

	job.baseUri = fmt.Sprintf("%s://%s:%s", httpUrl.Scheme, httpUrl.Hostname(), port)
	job.requestUri = httpUrl.RequestURI()

	return nil
}

func (job *Job) initHttpHeaders() (err error) {
	if len(job.HttpHeaders) > 0 {
		job.headers = make(http.Header, len(job.HttpHeaders))
	}

	var (
		h  string
		kv []string
	)

	for _, h = range job.HttpHeaders {
		kv = strings.SplitN(h, ":", 2)
		if len(kv) != 2 {
			return fmt.Errorf("%s: invalid header %q", job.ID, h)
		}

		job.headers.Set(strings.TrimSpace(kv[0]), strings.TrimSpace(kv[1]))
	}
	return nil
}

func (job *Job) increment() (ok bool) {
	job.Lock()
	if job.NumRequests+1 <= job.MaxRequests {
		job.NumRequests++
		ok = true
	}
	job.Unlock()
	return ok
}

func (job *Job) decrement() {
	job.Lock()
	job.NumRequests--
	job.Unlock()
}

func (job *Job) execute() {
	var (
		now     = time.Now().UTC().Round(time.Second)
		logTime = now.Format(defTimeLayout)

		httpRes *http.Response
		log     string
		err     error
		resBody []byte
	)

	if job.isPaused() {
		job.mlog.Outf(JobStatusPaused)
		job.Log.Push(fmt.Sprintf("%s %s: %s", logTime, job.ID, JobStatusPaused))
		job.LastRun = now
		return
	}

	if !job.increment() {
		log = fmt.Sprintf("!!! maximum requests %d has been reached", job.MaxRequests)
		job.mlog.Errf(log)
		job.Log.Push(fmt.Sprintf("%s %s: %s", logTime, job.ID, log))
		return
	}
	defer job.decrement()

	httpRes, resBody, err = job.httpc.Get(job.requestUri, nil, nil)
	if err != nil {
		log = fmt.Sprintf("!!! %s", err)
		job.mlog.Errf(log)
		job.Log.Push(fmt.Sprintf("%s %s: %s", logTime, job.ID, log))
		job.Status = JobStatusFailed
		job.LastRun = now
		return
	}

	if httpRes.StatusCode != http.StatusOK {
		log = fmt.Sprintf("!!! %s: %s", httpRes.Status, resBody)
		job.mlog.Errf(log)
		job.Log.Push(fmt.Sprintf("%s %s: %s", logTime, job.ID, log))
		job.Status = JobStatusFailed
		job.LastRun = now
		return
	}

	log = fmt.Sprintf(">>> %s\n", resBody)
	job.mlog.Outf(log)
	job.Log.Push(fmt.Sprintf("%s %s: %s", logTime, job.ID, log))
	job.Status = JobStatusSuccess
	job.LastRun = now
}

// computeFirstTimer compute the duration when the job will be running based
// on last time run and interval.
//
// If the `(last_run + interval) < now` then it will return 0; otherwise it will
// return `(last_run + interval) - now`
func (job *Job) computeFirstTimer(now time.Time) time.Duration {
	var lastInterval time.Time = job.LastRun.Add(job.Interval)
	if lastInterval.Before(now) {
		return 0
	}
	return lastInterval.Sub(now)
}

func (job *Job) pause() {
	job.mlog.Outf("pausing...\n")
	job.Lock()
	job.Status = JobStatusPaused
	job.Unlock()
}

func (job *Job) resume() {
	job.mlog.Outf("resuming...\n")
	job.Lock()
	job.Status = JobStatusStarted
	job.Unlock()
}

// stateLoad load the job state from file Job.pathState.
func (job *Job) stateLoad() (err error) {
	var rawState []byte

	rawState, err = os.ReadFile(job.pathState)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	err = job.jobState.unpack(rawState)
	if err != nil {
		return err
	}
	return nil
}

// stateSave save the job state into file job.pathState.
func (job *Job) stateSave() (err error) {
	var rawState []byte

	rawState, err = job.jobState.pack()
	if err != nil {
		return err
	}

	err = os.WriteFile(job.pathState, rawState, 0600)
	if err != nil {
		return err
	}
	return nil
}

func (job *Job) isPaused() (b bool) {
	job.Lock()
	b = job.Status == JobStatusPaused
	job.Unlock()
	return b
}
