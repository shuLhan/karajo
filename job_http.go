// SPDX-FileCopyrightText: 2021 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

package karajo

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	libhttp "github.com/shuLhan/share/lib/http"
	"github.com/shuLhan/share/lib/mlog"
)

const (
	defJobHttpMethod = http.MethodGet
	defJobInterval   = 30 * time.Second
	defJosParamEpoch = "_karajo_epoch"

	defTimeLayout = "2006-01-02 15:04:05 MST"
)

// JobHttp A JobHttp is a periodic job that send HTTP request to external HTTP
// server (or to karajo Job itself).
//
// See the [JobBase]'s Interval and Schedule fields for more information on
// how to setup periodic time.
//
// Each JobHttp execution send the parameter named `_karajo_epoch` with value
// is current server Unix time.
// If the request type is `query` then the parameter is inside the query URL.
// If the request type is `form` then the parameter is inside the body.
// If the request type is `json` then the parameter is inside the body as JSON
// object, for example `{"_karajo_epoch":1656750073}`.
type JobHttp struct {
	headers http.Header

	// httpc define the HTTP client that will execute the http_url.
	httpc *libhttp.Client

	params map[string]interface{}

	startq chan struct{}
	stopq  chan struct{}

	// Secret define a string to sign the request query or body with
	// HMAC+SHA-256.
	// The signature is sent on HTTP header "X-Karajo-Sign" as hex string.
	// This field is optional.
	Secret string `ini:"::secret" json:"-"`

	// HeaderSign define the HTTP header where the signature will be
	// written in request.
	// Default to "X-Karajo-Sign" if its empty.
	HeaderSign string `ini:"::header_sign" json:"header_sign,omitempty"`

	// HttpMethod HTTP method to be used in request for job execution.
	// Its accept only GET, POST, PUT, or DELETE.
	// This field is optional, default to GET.
	HttpMethod string `ini:"::http_method" json:"http_method"`

	// The HTTP URL where the job will be executed.
	// This field is required.
	HttpUrl    string `ini:"::http_url" json:"http_url"`
	baseUri    string
	requestUri string

	// HttpRequestType The header Content-Type to be set on request.
	//
	//   - (empty string): no header Content-Type set.
	//   - query: no header Content-Type to be set, reserved for future
	//   use.
	//   - form: header Content-Type set to
	//   "application/x-www-form-urlencoded".
	//   - json: header Content-Type set to "application/json".
	//
	// The type "form" and "json" only applicable if the HttpMethod is
	// POST or PUT.
	// This field is optional, default to query.
	HttpRequestType string `ini:"::http_request_type" json:"http_request_type"`

	// Optional HTTP headers for HttpUrl, in the format of "K: V".
	HttpHeaders []string `ini:"::http_header" json:"http_headers,omitempty"`

	JobBase

	// HttpTimeout custom HTTP timeout for this job.
	// This field is optional, if not set default to global timeout in
	// Environment.HttpTimeout.
	// To make job run without timeout, set the value to negative.
	HttpTimeout time.Duration `ini:"::http_timeout" json:"http_timeout"`

	requestMethod libhttp.RequestMethod
	requestType   libhttp.RequestType

	// HttpInsecure can be set to true if the http_url is HTTPS with
	// unknown Certificate Authority.
	HttpInsecure bool `ini:"::http_insecure" json:"http_insecure,omitempty"`
}

func (job *JobHttp) Start() {
	if job.scheduler != nil {
		job.startScheduler()
		return
	}
	if job.Interval > 0 {
		job.startInterval()
	}
}

func (job *JobHttp) startScheduler() {
	var (
		jlog *JobLog
		err  error
	)

	for {
		select {
		case <-job.scheduler.C:
			select {
			case job.startq <- struct{}{}:
			default:
			}

		case <-job.startq:
			err = job.start()
			if err != nil {
				mlog.Errf(`!!! job_http: %s: %s`, job.ID, err)
				continue
			}

			jlog, err = job.execute()
			if err != nil {
				mlog.Errf(`!!! job_http: %s: failed: %s.`, job.ID, err)
			} else {
				mlog.Outf(`job_http: %s: finished.`, job.ID)
			}
			job.finish(jlog, err)

			select {
			case job.finishq <- struct{}{}:
			default:
			}

		case <-job.stopq:
			job.scheduler.Stop()
			return
		}
	}
}

func (job *JobHttp) startInterval() {
	var (
		now          time.Time
		nextInterval time.Duration
		timer        *time.Timer
		jlog         *JobLog
		err          error
		ever         bool
	)

	for {
		job.Lock()
		now = TimeNow().UTC().Round(time.Second)
		nextInterval = job.computeNextInterval(now)
		job.NextRun = now.Add(nextInterval)
		job.Unlock()

		mlog.Outf(`next running in %s ...`, nextInterval)

		timer = time.NewTimer(nextInterval)
		ever = true
		for ever {
			select {
			case <-timer.C:
				select {
				case job.startq <- struct{}{}:
				default:
				}

			case <-job.startq:
				err = job.start()
				if err != nil {
					mlog.Errf(`!!! %s`, err)
					timer.Stop()
					ever = false
					continue
				}

				jlog, err = job.execute()
				if err != nil {
					mlog.Errf(`!!! %s`, err)
				} else {
					mlog.Outf(`finished`)
				}
				job.finish(jlog, err)

				timer.Stop()
				ever = false

				select {
				case job.finishq <- struct{}{}:
				default:
				}

			case <-job.stopq:
				timer.Stop()
				return
			}
		}
	}
}

// Stop the job.
func (job *JobHttp) Stop() {
	mlog.Outf(`stopping HTTP job ...`)
	select {
	case job.stopq <- struct{}{}:
	default:
	}

	mlog.Flush()
}

// init initialize the job, compute the last run and the next run.
func (job *JobHttp) init(env *Environment, name string) (err error) {
	var logp = `init`

	job.startq = make(chan struct{}, 1)
	job.stopq = make(chan struct{}, 1)

	job.JobBase.init(name)

	err = job.initDirsState(env)
	if err != nil {
		return fmt.Errorf(`%s: %w`, logp, err)
	}

	err = job.JobBase.initLogs()
	if err != nil {
		return fmt.Errorf(`%s: %w`, logp, err)
	}

	err = job.initHttpMethod()
	if err != nil {
		return err
	}

	err = job.initHttpRequestType()
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

	job.params = make(map[string]interface{})

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

	if len(job.HeaderSign) == 0 {
		job.HeaderSign = HeaderNameXKarajoSign
	}

	err = job.JobBase.initTimer()
	if err != nil {
		return fmt.Errorf(`%s: %w`, logp, err)
	}

	return nil
}

func (job *JobHttp) initDirsState(env *Environment) (err error) {
	job.dirWork = filepath.Join(env.dirLibJobHttp, job.ID)
	err = os.MkdirAll(job.dirWork, 0700)
	if err != nil {
		return err
	}

	job.dirLog = filepath.Join(env.dirLogJobHttp, job.ID)

	// Remove previous log file.
	_ = os.Remove(job.dirLog)

	err = os.MkdirAll(job.dirLog, 0700)
	if err != nil {
		return err
	}

	return nil
}

// initHttpMethod check if defined HTTP method is valid.
// If its empty, set default to GET, otherwise return an error.
func (job *JobHttp) initHttpMethod() (err error) {
	job.HttpMethod = strings.TrimSpace(job.HttpMethod)
	if len(job.HttpMethod) == 0 {
		job.HttpMethod = defJobHttpMethod
		job.requestMethod = libhttp.RequestMethodGet
		return nil
	}

	var vstr = strings.ToUpper(job.HttpMethod)

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

func (job *JobHttp) initHttpRequestType() (err error) {
	var vstr = strings.ToLower(job.HttpRequestType)
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

func (job *JobHttp) initHttpUrl(serverAddress string) (err error) {
	if job.HttpUrl[0] == '/' {
		job.baseUri = fmt.Sprintf(`http://%s`, serverAddress)
		job.requestUri = job.HttpUrl
		return nil
	}

	var (
		httpUrl *url.URL
		port    string
	)

	httpUrl, err = url.Parse(job.HttpUrl)
	if err != nil {
		return fmt.Errorf(`%s: invalid http_url %q: %w`, job.ID, job.HttpUrl, err)
	}

	port = httpUrl.Port()
	if len(port) == 0 {
		if httpUrl.Scheme == `https` {
			port = `443`
		} else {
			port = `80`
		}
	}

	job.baseUri = fmt.Sprintf(`%s://%s:%s`, httpUrl.Scheme, httpUrl.Hostname(), port)
	job.requestUri = httpUrl.RequestURI()

	return nil
}

func (job *JobHttp) initHttpHeaders() (err error) {
	if len(job.HttpHeaders) > 0 {
		job.headers = make(http.Header, len(job.HttpHeaders))
	}

	var (
		h  string
		kv []string
	)

	for _, h = range job.HttpHeaders {
		kv = strings.SplitN(h, `:`, 2)
		if len(kv) != 2 {
			return fmt.Errorf(`%s: invalid header %q`, job.ID, h)
		}

		job.headers.Set(strings.TrimSpace(kv[0]), strings.TrimSpace(kv[1]))
	}
	return nil
}

func (job *JobHttp) execute() (jlog *JobLog, err error) {
	var (
		logp    = `execute`
		now     = TimeNow().UTC().Round(time.Second)
		logTime = now.Format(defTimeLayout)
		headers = http.Header{}

		params  interface{}
		httpReq *http.Request
		httpRes *http.Response
		rawb    []byte
	)

	job.setStatus(JobStatusRunning)
	job.lastCounter++
	jlog = newJobLog(job.ID, job.dirLog, job.lastCounter)
	job.Logs = append(job.Logs, jlog)
	job.logsPrune()

	fmt.Fprintf(jlog, "%s === BEGIN\n", logTime)

	job.params[defJosParamEpoch] = now.Unix()

	switch job.requestType {
	case libhttp.RequestTypeQuery, libhttp.RequestTypeForm:
		params, rawb = job.paramsToUrlValues()

	case libhttp.RequestTypeJSON:
		params, rawb, err = job.paramsToJson()
		if err != nil {
			return jlog, fmt.Errorf(`%s: %w`, logp, err)
		}
	}

	if len(job.Secret) != 0 {
		var sign = Sign(rawb, []byte(job.Secret))
		headers.Set(job.HeaderSign, sign)
	}

	httpReq, err = job.httpc.GenerateHttpRequest(job.requestMethod, job.requestUri, job.requestType, headers, params)
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

	fmt.Fprintf(jlog, "=== DONE\n")

	return jlog, nil
}

func (job *JobHttp) paramsToJson() (obj map[string]interface{}, raw []byte, err error) {
	raw, err = json.Marshal(job.params)
	if err != nil {
		return nil, nil, err
	}
	return job.params, raw, nil
}

// paramsToUrlValues convert the job parameters to url.Values.
func (job *JobHttp) paramsToUrlValues() (url.Values, []byte) {
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

func (job *JobHttp) setStatus(status string) {
	job.Lock()
	job.Status = status
	job.Unlock()
}
