// SPDX-FileCopyrightText: 2021 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

package karajo

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	libhttp "github.com/shuLhan/share/lib/http"
	"github.com/shuLhan/share/lib/mlog"
)

const (
	defJobHTTPMethod = http.MethodGet
	defJobInterval   = 30 * time.Second
	defJosParamEpoch = "_karajo_epoch"

	defTimeLayout = "2006-01-02 15:04:05 MST"
)

// JobHTTP A JobHTTP is a periodic job that send HTTP request to external HTTP
// server (or to karajo Job itself).
//
// See the [JobBase]'s Interval and Schedule fields for more information on
// how to setup periodic time.
//
// Each JobHTTP execution send the parameter named "_karajo_epoch" with value
// set to current server Unix timestamp.
// If the request type is "query" then the parameter is inside the query URL.
// If the request type is "form" then the parameter is inside the body.
// If the request type is "json" then the parameter is inside the body as JSON
// object, for example '{"_karajo_epoch":1656750073}'.
//
// The job configuration in INI format,
//
//	[job "name"]
//	secret =
//	header_sign =
//	http_method =
//	http_url =
//	http_request_type =
//	http_header =
//	http_timeout =
//	http_insecure =
type JobHTTP struct {
	// jobq is a channel passed by Karajo instance to limit number of
	// job running at the same time.
	jobq chan struct{}

	headers http.Header

	// httpc define the HTTP client that will execute the http_url.
	httpc *libhttp.Client

	params map[string]interface{}

	stopq chan struct{}

	// Secret define a string to sign the request query or body with
	// HMAC+SHA-256.
	// The signature is sent on HTTP header "X-Karajo-Sign" as hex string.
	// This field is optional.
	Secret string `ini:"::secret" json:"-"`

	// HeaderSign define the HTTP header where the signature will be
	// written in request.
	// Default to "X-Karajo-Sign" if its empty.
	HeaderSign string `ini:"::header_sign" json:"header_sign,omitempty"`

	// HTTPMethod HTTP method to be used in request for job execution.
	// Its accept only GET, POST, PUT, or DELETE.
	// This field is optional, default to GET.
	HTTPMethod string `ini:"::http_method" json:"http_method"`

	// The HTTP URL where the job will be executed.
	// This field is required.
	HTTPURL    string `ini:"::http_url" json:"http_url"`
	baseURI    string
	requestURI string

	// HTTPRequestType The header Content-Type to be set on request.
	//
	//   - (empty string): no header Content-Type set.
	//   - query: no header Content-Type to be set, reserved for future
	//   use.
	//   - form: header Content-Type set to
	//   "application/x-www-form-urlencoded".
	//   - json: header Content-Type set to "application/json".
	//
	// The type "form" and "json" only applicable if the HTTPMethod is
	// POST or PUT.
	// This field is optional, default to query.
	HTTPRequestType string `ini:"::http_request_type" json:"http_request_type"`

	// Optional HTTP headers for HTTPURL, in the format of "K: V".
	HTTPHeaders []string `ini:"::http_header" json:"http_headers,omitempty"`

	JobBase

	// HTTPTimeout custom HTTP timeout for this job.
	// This field is optional, if not set default to global timeout in
	// Env.HTTPTimeout.
	// To make job run without timeout, set the value to negative.
	HTTPTimeout time.Duration `ini:"::http_timeout" json:"http_timeout"`

	requestMethod libhttp.RequestMethod
	requestType   libhttp.RequestType

	// HTTPInsecure can be set to true if the http_url is HTTPS with
	// unknown Certificate Authority.
	HTTPInsecure bool `ini:"::http_insecure" json:"http_insecure,omitempty"`
}

// Start running the job.
func (job *JobHTTP) Start(jobq chan struct{}, logq chan<- *JobLog) {
	job.jobq = jobq
	job.JobBase.logq = logq

	if job.scheduler != nil {
		job.startScheduler()
		return
	}
	if job.Interval > 0 {
		job.startInterval()
	}
}

func (job *JobHTTP) startScheduler() {
	for {
		select {
		case <-job.scheduler.C:
			job.run()

		case <-job.stopq:
			job.scheduler.Stop()
			return
		}
	}
}

func (job *JobHTTP) startInterval() {
	var (
		now          time.Time
		nextInterval time.Duration
		timer        *time.Timer
	)

	for {
		job.Lock()
		now = TimeNow().UTC().Round(time.Second)
		nextInterval = job.computeNextInterval(now)
		job.NextRun = now.Add(nextInterval)
		job.Unlock()

		if timer == nil {
			timer = time.NewTimer(nextInterval)
		} else {
			timer.Reset(nextInterval)
		}

		select {
		case <-timer.C:

		case <-job.stopq:
			timer.Stop()
			return
		}

		timer.Stop()
		job.run()
	}
}

func (job *JobHTTP) run() {
	var (
		jlog *JobLog
		err  error
	)

	jlog, err = job.execute()
	job.finish(jlog, err)
}

// Stop the job.
func (job *JobHTTP) Stop() {
	mlog.Outf(`%s: %s: stopping ...`, job.kind, job.ID)
	select {
	case job.stopq <- struct{}{}:
	default:
	}

	mlog.Flush()
}

// init initialize the job, compute the last run and the next run.
func (job *JobHTTP) init(env *Env, name string) (err error) {
	var logp = `init`

	job.stopq = make(chan struct{}, 1)
	job.JobBase.kind = jobKindHTTP

	err = job.JobBase.init(env, name)
	if err != nil {
		return fmt.Errorf(`%s: %w`, logp, err)
	}

	err = job.initHTTPMethod()
	if err != nil {
		return err
	}

	err = job.initHTTPRequestType()
	if err != nil {
		return err
	}

	err = job.initHTTPURL(env.ListenAddress)
	if err != nil {
		return err
	}

	err = job.initHTTPHeaders()
	if err != nil {
		return err
	}

	job.params = make(map[string]interface{})

	var httpClientOpts = &libhttp.ClientOptions{
		ServerUrl:     job.baseURI,
		Headers:       job.headers,
		AllowInsecure: job.HTTPInsecure,
	}
	job.httpc = libhttp.NewClient(httpClientOpts)

	if job.HTTPTimeout == 0 {
		job.HTTPTimeout = env.HTTPTimeout
	} else if job.HTTPTimeout < 0 {
		// Negative value means 0 on net/http.Client.
		job.HTTPTimeout = 0
	}
	job.httpc.Client.Timeout = job.HTTPTimeout

	if len(job.HeaderSign) == 0 {
		job.HeaderSign = HeaderNameXKarajoSign
	}

	return nil
}

// initHTTPMethod check if defined HTTP method is valid.
// If its empty, set default to GET, otherwise return an error.
func (job *JobHTTP) initHTTPMethod() (err error) {
	job.HTTPMethod = strings.TrimSpace(job.HTTPMethod)
	if len(job.HTTPMethod) == 0 {
		job.HTTPMethod = defJobHTTPMethod
		job.requestMethod = libhttp.RequestMethodGet
		return nil
	}

	var vstr = strings.ToUpper(job.HTTPMethod)

	switch vstr {
	case http.MethodGet:
		job.requestMethod = libhttp.RequestMethodGet
	case http.MethodDelete:
		job.requestMethod = libhttp.RequestMethodDelete
	case http.MethodPost:
		job.requestMethod = libhttp.RequestMethodPost
	case http.MethodPut:
		job.requestMethod = libhttp.RequestMethodPut
	default:
		return fmt.Errorf(`invalid HTTP method %q`, vstr)
	}
	return nil
}

func (job *JobHTTP) initHTTPRequestType() (err error) {
	var vstr = strings.ToLower(job.HTTPRequestType)
	switch vstr {
	case ``, `query`:
		job.requestType = libhttp.RequestTypeQuery
	case `form`:
		job.requestType = libhttp.RequestTypeForm
	case `json`:
		job.requestType = libhttp.RequestTypeJSON
	default:
		return fmt.Errorf(`invalid HTTP request type %q`, vstr)
	}
	return nil
}

func (job *JobHTTP) initHTTPURL(serverAddress string) (err error) {
	if job.HTTPURL[0] == '/' {
		job.baseURI = fmt.Sprintf(`http://%s`, serverAddress)
		job.requestURI = job.HTTPURL
		return nil
	}

	var (
		httpURL *url.URL
		port    string
	)

	httpURL, err = url.Parse(job.HTTPURL)
	if err != nil {
		return fmt.Errorf(`%s: invalid http_url %q: %w`, job.ID, job.HTTPURL, err)
	}

	port = httpURL.Port()
	if len(port) == 0 {
		if httpURL.Scheme == `https` {
			port = `443`
		} else {
			port = `80`
		}
	}

	job.baseURI = fmt.Sprintf(`%s://%s:%s`, httpURL.Scheme, httpURL.Hostname(), port)
	job.requestURI = httpURL.RequestURI()

	return nil
}

func (job *JobHTTP) initHTTPHeaders() (err error) {
	if len(job.HTTPHeaders) > 0 {
		job.headers = make(http.Header, len(job.HTTPHeaders))
	}

	var (
		h  string
		kv []string
	)

	for _, h = range job.HTTPHeaders {
		kv = strings.SplitN(h, `:`, 2)
		if len(kv) != 2 {
			return fmt.Errorf(`%s: invalid header %q`, job.ID, h)
		}

		job.headers.Set(strings.TrimSpace(kv[0]), strings.TrimSpace(kv[1]))
	}
	return nil
}

func (job *JobHTTP) execute() (jlog *JobLog, err error) {
	jlog = job.JobBase.newLog()
	if jlog.Status == JobStatusPaused {
		return jlog, nil
	}

	var (
		logp    = `execute`
		now     = TimeNow().UTC().Round(time.Second)
		headers = http.Header{}

		params  interface{}
		httpReq *http.Request
		httpRes *http.Response
		rawb    []byte
	)

	_, _ = jlog.Write([]byte("=== BEGIN\n"))

	job.params[defJosParamEpoch] = now.Unix()

	switch job.requestType {
	case libhttp.RequestTypeQuery, libhttp.RequestTypeForm:
		params, rawb = job.paramsToURLValues()

	case libhttp.RequestTypeJSON:
		params, rawb, err = job.paramsToJSON()
		if err != nil {
			return jlog, fmt.Errorf(`%s: %w`, logp, err)
		}
	}

	if len(job.Secret) != 0 {
		var sign = Sign(rawb, []byte(job.Secret))
		headers.Set(job.HeaderSign, sign)
	}

	httpReq, err = job.httpc.GenerateHttpRequest(job.requestMethod, job.requestURI, job.requestType, headers, params)
	if err != nil {
		return jlog, fmt.Errorf(`%s: %w`, logp, err)
	}

	rawb, err = httputil.DumpRequestOut(httpReq, true)
	if err != nil {
		return jlog, fmt.Errorf(`%s: %w`, logp, err)
	}

	fmt.Fprintf(jlog, "--- HTTP request:\n%s\n\n", rawb)

	httpRes, _, err = job.httpc.Do(httpReq)
	if err != nil {
		return jlog, fmt.Errorf(`%s: %w`, logp, err)
	}

	rawb, err = httputil.DumpResponse(httpRes, true)
	if err != nil {
		return jlog, fmt.Errorf(`%s: %w`, logp, err)
	}

	fmt.Fprintf(jlog, "--- HTTP response:\n%s\n\n", rawb)

	if httpRes.StatusCode != http.StatusOK {
		return jlog, fmt.Errorf(`%s: %s`, logp, httpRes.Status)
	}

	_, _ = jlog.Write([]byte("=== DONE\n"))

	return jlog, nil
}

func (job *JobHTTP) paramsToJSON() (obj map[string]interface{}, raw []byte, err error) {
	raw, err = json.Marshal(job.params)
	if err != nil {
		return nil, nil, err
	}
	return job.params, raw, nil
}

// paramsToURLValues convert the job parameters to url.Values.
func (job *JobHTTP) paramsToURLValues() (url.Values, []byte) {
	var (
		urlValues = url.Values{}

		k string
		v interface{}
	)
	for k, v = range job.params {
		urlValues.Set(k, fmt.Sprintf(`%s`, v))
	}
	return urlValues, []byte(urlValues.Encode())
}

func (job *JobHTTP) setStatus(status string) {
	job.Lock()
	job.Status = status
	job.Unlock()
}
