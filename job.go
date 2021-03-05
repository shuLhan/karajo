// Copyright 2021, Shulhan <ms@kilabit.info>. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package karajo

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/shuLhan/share/lib/clise"
	libhttp "github.com/shuLhan/share/lib/http"
	"github.com/shuLhan/share/lib/mlog"
)

// List of job status.
const (
	JobStatusStarted = 0
	JobStatusSuccess = 1
	JobStatusFailed  = 2
)

// DefaultMaxRequests define maximum number of requests that can be
// executed simultaneously.
// This is to prevent the karajo server consume resources on the local
// server and on the remote server.
const DefaultMaxRequests = 1

const (
	defJobDelay    = 30 * time.Second
	defJobLogsSize = 20
	defTimeLayout  = "2006-01-02 15:04:05 MST"
)

//
// Job is the worker that will trigger HTTP GET request to the remote job
// periodically and save the response status to logs and the last execution
// time for future run.
//
type Job struct {
	sync.Mutex

	// ID of the job. It must be unique or the last job will replace the
	// previous job with the same ID.
	// If ID is empty, it will generated from Name by replacing
	// non-alphanumeric character with '-'.
	ID string

	// Name of job for readibility.
	Name string `ini:"::name"`

	//
	// The HTTP URL where the job will be executed.
	// This field is required.
	//
	HttpUrl    string `ini:"::http_url"`
	baseUri    string
	requestUri string

	// Optional HTTP headers for HttpUrl, in the format of "K: V".
	HttpHeaders []string `ini:"::http_header"`
	headers     http.Header

	//
	// HttpInsecure can be set to true if the http_url is HTTPS with
	// unknown certificate authority.
	//
	HttpInsecure bool `ini:"::http_insecure"`

	//
	// Delay when job will be repeatedly executed.
	// This field is required, if not set or invalid it will set to 10
	// minutes.
	//
	// If one have job that need to run every less than 10 minutes, it
	// should be run on single program.
	//
	Delay time.Duration `ini:"::delay"`

	// MaxRequests maximum number of requests executed by karajo.
	// If zero, it will set to DefaultMaxRequests.
	MaxRequests int `ini:"::max_requests"`
	numRequests int

	// The last time the job is running, in UTC.
	LastRun time.Time
	// The next time the job will running, in UTC.
	NextRun time.Time

	// The last status of execute, 0 for success and 1 for fail.
	LastStatus int

	// httpc define the HTTP client that will execute the http_url.
	httpc *libhttp.Client

	// logs contains 100 last jobs output.
	logs *clise.Clise

	done chan bool
}

func (job *Job) Start() (err error) {
	mlog.Outf("job %s: %s: %+v\n", job.Name, job.HttpUrl, job)

	lastRunTs := job.LastRun.Unix()
	nextSeconds := lastRunTs % int64(job.Delay.Seconds())

	firstTimer := time.Duration(nextSeconds) * time.Second
	job.NextRun = time.Now().Add(firstTimer).UTC()
	mlog.Outf("%s: running the first job in %s ...\n", job.Name, firstTimer)

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

	job.NextRun = job.LastRun.Add(job.Delay).UTC()
	mlog.Outf("%s: running the next job at %s ...\n", job.Name,
		job.NextRun.Format(defTimeLayout))

	tick := time.NewTicker(job.Delay)
	for {
		select {
		case <-tick.C:
			job.execute()
			job.NextRun = job.LastRun.Add(job.Delay).UTC()
			mlog.Outf("%s: running the next job at %s\n", job.Name,
				job.NextRun.Format(defTimeLayout))
		case <-job.done:
			return nil
		}
	}

	return nil
}

//
// Stop the job.
//
func (job *Job) Stop() {
	job.done <- true
}

//
// init initialize the job, compute the last run and the next run.
//
func (job *Job) init(serverAddress string) (err error) {
	if len(job.ID) == 0 {
		job.generateID()
	}

	err = job.initHttpUrl(serverAddress)
	if err != nil {
		return err
	}

	err = job.initHttpHeaders()
	if err != nil {
		return err
	}

	job.httpc = libhttp.NewClient(job.baseUri, job.headers, job.HttpInsecure)
	job.logs = clise.New(defJobLogsSize)

	if job.Delay <= defJobDelay {
		job.Delay = defJobDelay
	}
	if job.MaxRequests == 0 {
		job.MaxRequests = DefaultMaxRequests
	}

	job.done = make(chan bool)

	return nil
}

//
// generateID generate unique job ID based on job's Name.
// Any non-alphanumeric characters in job name will be replaced with '-'.
//
func (job *Job) generateID() {
	id := make([]rune, 0, len(job.Name))
	for _, r := range strings.ToLower(job.Name) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			id = append(id, r)
		} else {
			id = append(id, '-')
		}
	}
	job.ID = string(id)
}

func (job *Job) initHttpUrl(serverAddress string) (err error) {
	if job.HttpUrl[0] == '/' {
		job.baseUri = fmt.Sprintf("http://%s", serverAddress)
		job.requestUri = job.HttpUrl
	} else {
		httpUrl, err := url.Parse(job.HttpUrl)
		if err != nil {
			return fmt.Errorf("job %s: invalid http_url %q: %w",
				job.Name, job.HttpUrl, err)
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
			return fmt.Errorf("job %s: invalid header %q",
				job.Name, h)
		}

		job.headers.Set(strings.TrimSpace(kv[0]), strings.TrimSpace(kv[1]))
	}
	return nil
}

func (job *Job) increment() (ok bool) {
	job.Lock()
	job.numRequests++
	if job.numRequests <= job.MaxRequests {
		ok = true
	}
	job.Unlock()
	return ok
}

func (job *Job) decrement() {
	job.Lock()
	job.numRequests--
	job.Unlock()
}

func (job *Job) execute() {
	now := time.Now().UTC()
	logTime := now.Format(defTimeLayout)

	if !job.increment() {
		log := fmt.Sprintf("!!! %s %s: maximum requests %d has been reached",
			logTime, job.Name, job.MaxRequests)
		mlog.Errf(log)
		job.logs.Push(log)
		return
	}
	defer job.decrement()

	httpRes, resBody, err := job.httpc.Get(nil, job.requestUri, nil)

	if err != nil {
		log := fmt.Sprintf("!!! %s %s: %s", logTime, job.Name, err)
		mlog.Errf(log)
		job.logs.Push(log)
		job.LastStatus = JobStatusFailed
		job.LastRun = now
		return
	}

	if httpRes.StatusCode != http.StatusOK {
		log := fmt.Sprintf("!!! %s %s: %s", logTime, job.Name, httpRes.Status)
		mlog.Errf(log)
		job.logs.Push(log)
		job.LastStatus = JobStatusFailed
		job.LastRun = now
		return
	}

	log := fmt.Sprintf("--- %s %s: %s\n", logTime, job.Name, resBody)
	mlog.Outf(log)
	job.logs.Push(log)
	job.LastStatus = JobStatusSuccess
	job.LastRun = now
}
