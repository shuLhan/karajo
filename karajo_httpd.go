// SPDX-FileCopyrightText: 2021 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

package karajo

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	liberrors "github.com/shuLhan/share/lib/errors"
	libhttp "github.com/shuLhan/share/lib/http"
	"github.com/shuLhan/share/lib/memfs"
)

// HeaderNameXKarajoSign the header key for the signature of body.
const HeaderNameXKarajoSign = `X-Karajo-Sign`

// List of HTTP API.
const (
	apiAuthLogin = `/karajo/api/auth/login`

	apiEnvironment = `/karajo/api/environment`

	apiJobHttp       = `/karajo/api/job_http`
	apiJobHttpLog    = `/karajo/api/job_http/log`
	apiJobHttpPause  = `/karajo/api/job_http/pause`
	apiJobHttpResume = `/karajo/api/job_http/resume`

	apiJobLog    = `/karajo/api/job/log`
	apiJobPause  = `/karajo/api/job/pause`
	apiJobResume = `/karajo/api/job/resume`
	apiJobRun    = `/karajo/api/job/run`
)

// List of known pathes.
const (
	pathKarajoApi = `/karajo/api/`
	pathKarajoApp = `/karajo/app/`
)

// List of known HTTP request parameters.
const (
	paramNameCounter     = `counter`
	paramNameID          = `id`
	paramNameKarajoEpoch = `_karajo_epoch`
	paramNameName        = `name`
	paramNamePassword    = `password`
)

// List of errors related to HTTP APIs.
var (
	errAuthLogin = liberrors.E{
		Code:    http.StatusBadRequest,
		Name:    `ERR_AUTH_LOGIN`,
		Message: `invalid user name and/or password`,
	}
)

// initHttpd initialize the HTTP server, including registering its endpoints
// and Job endpoints.
func (k *Karajo) initHttpd() (err error) {
	var (
		logp       = `initHttpd`
		serverOpts = libhttp.ServerOptions{
			Address: k.env.ListenAddress,
			Conn: &http.Server{
				ReadTimeout:    10 * time.Minute,
				WriteTimeout:   10 * time.Minute,
				MaxHeaderBytes: 1 << 20,
			},
			HandleFS:        k.handleFSAuth,
			Memfs:           memfsWww,
			EnableIndexHtml: true,
		}
	)

	k.httpd, err = libhttp.NewServer(&serverOpts)
	if err != nil {
		return fmt.Errorf(`%s: %w`, logp, err)
	}

	err = k.registerApis()
	if err != nil {
		return fmt.Errorf(`%s: %w`, logp, err)
	}

	err = k.registerJobsHook()
	if err != nil {
		return fmt.Errorf(`%s: %w`, logp, err)
	}

	return nil
}

// registerApis register the public HTTP APIs.
func (k *Karajo) registerApis() (err error) {
	var logp = `registerApis`

	err = k.httpd.RegisterEndpoint(&libhttp.Endpoint{
		Method:       libhttp.RequestMethodPost,
		Path:         apiAuthLogin,
		RequestType:  libhttp.RequestTypeForm,
		ResponseType: libhttp.ResponseTypeJSON,
		Call:         k.apiAuthLogin,
	})
	if err != nil {
		return fmt.Errorf(`%s: %w`, logp, err)
	}

	err = k.httpd.RegisterEndpoint(&libhttp.Endpoint{
		Method:       libhttp.RequestMethodGet,
		Path:         apiEnvironment,
		RequestType:  libhttp.RequestTypeNone,
		ResponseType: libhttp.ResponseTypeJSON,
		Call:         k.apiEnvironment,
	})
	if err != nil {
		return err
	}

	err = k.httpd.RegisterEndpoint(&libhttp.Endpoint{
		Method:       libhttp.RequestMethodGet,
		Path:         apiJobLog,
		RequestType:  libhttp.RequestTypeQuery,
		ResponseType: libhttp.ResponseTypeJSON,
		Call:         k.apiJobLog,
	})
	if err != nil {
		return err
	}
	err = k.httpd.RegisterEndpoint(&libhttp.Endpoint{
		Method:       libhttp.RequestMethodPost,
		Path:         apiJobPause,
		RequestType:  libhttp.RequestTypeForm,
		ResponseType: libhttp.ResponseTypeJSON,
		Call:         k.apiJobPause,
	})
	if err != nil {
		return fmt.Errorf(`%s: %s: %w`, logp, apiJobPause, err)
	}
	err = k.httpd.RegisterEndpoint(&libhttp.Endpoint{
		Method:       libhttp.RequestMethodPost,
		Path:         apiJobResume,
		RequestType:  libhttp.RequestTypeForm,
		ResponseType: libhttp.ResponseTypeJSON,
		Call:         k.apiJobResume,
	})
	if err != nil {
		return fmt.Errorf(`%s: %s: %w`, logp, apiJobResume, err)
	}

	err = k.httpd.RegisterEndpoint(&libhttp.Endpoint{
		Method:       libhttp.RequestMethodGet,
		Path:         apiJobHttp,
		RequestType:  libhttp.RequestTypeQuery,
		ResponseType: libhttp.ResponseTypeJSON,
		Call:         k.apiJobHttp,
	})
	if err != nil {
		return err
	}
	err = k.httpd.RegisterEndpoint(&libhttp.Endpoint{
		Method:       libhttp.RequestMethodGet,
		Path:         apiJobHttpLog,
		RequestType:  libhttp.RequestTypeQuery,
		ResponseType: libhttp.ResponseTypeJSON,
		Call:         k.apiJobHttpLog,
	})
	if err != nil {
		return err
	}
	err = k.httpd.RegisterEndpoint(&libhttp.Endpoint{
		Method:       libhttp.RequestMethodPost,
		Path:         apiJobHttpPause,
		RequestType:  libhttp.RequestTypeQuery,
		ResponseType: libhttp.ResponseTypeJSON,
		Call:         k.apiJobHttpPause,
	})
	if err != nil {
		return err
	}
	err = k.httpd.RegisterEndpoint(&libhttp.Endpoint{
		Method:       libhttp.RequestMethodPost,
		Path:         apiJobHttpResume,
		RequestType:  libhttp.RequestTypeQuery,
		ResponseType: libhttp.ResponseTypeJSON,
		Call:         k.apiJobHttpResume,
	})
	if err != nil {
		return err
	}

	return nil
}

// registerJobsHook register endpoint for executing Job using HTTP POST.
func (k *Karajo) registerJobsHook() (err error) {
	var job *Job

	for _, job = range k.env.Jobs {
		if len(job.Path) == 0 {
			// Ignore any job that does not have path.
			continue
		}

		err = k.httpd.RegisterEndpoint(&libhttp.Endpoint{
			Method:       libhttp.RequestMethodPost,
			Path:         path.Join(apiJobRun, job.Path),
			RequestType:  libhttp.RequestTypeJSON,
			ResponseType: libhttp.ResponseTypeJSON,
			Call:         job.handleHttp,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// handleFSAuth authorize access to resource based on the request path and
// cookie.
// If env.Users is empty, all request are accepted.
func (k *Karajo) handleFSAuth(_ *memfs.Node, w http.ResponseWriter, req *http.Request) bool {
	var path = req.URL.Path

	if k.isAuthorized(req) {
		if isLoginPage(path) {
			// Redirect user to app page if cookie is valid and
			// user in login page.
			http.Redirect(w, req, pathKarajoApp, http.StatusFound)
			return false
		}
		return true
	}
	if isRequireAuth(path) {
		return k.unauthorized(w, req)
	}

	return true
}

// isAuthorized return true env.Users is empty OR if the cookie exist and
// valid.
func (k *Karajo) isAuthorized(req *http.Request) bool {
	if len(k.env.Users) == 0 {
		return true
	}

	var (
		cookie *http.Cookie
		err    error
	)
	cookie, err = req.Cookie(cookieName)
	if err != nil {
		return false
	}

	var user = k.sm.get(cookie.Value)
	return user != nil
}

func isRequireAuth(path string) bool {
	if strings.HasPrefix(path, pathKarajoApp) {
		return true
	}
	if strings.HasPrefix(path, pathKarajoApi) {
		return true
	}
	return false
}

func isLoginPage(path string) bool {
	return path == `/karajo` || path == `/karajo/` || path == `/karajo/index.html`
}

// unauthorized write HTTP status 401 Unauthorized and return false.
func (k *Karajo) unauthorized(w http.ResponseWriter, req *http.Request) bool {
	w.WriteHeader(http.StatusUnauthorized)
	fmt.Fprintf(w, `Unauthorized`)
	return false
}

// apiAuthLogin authenticate user using name and password.
//
// A valid user's account will receive authorization cookie named `karajo`
// that can be used as authorization for subsequent request.
//
// Request format,
//
//	POST /karajo/api/auth/login
//	Content-Type: application/x-www-form-urlencoded
//
//	name=&password=
//
// List of response,
//
//   - 200 OK: success.
//   - 400 ERR_AUTH_LOGIN: invalid name and/or password.
//   - 500 ERR_INTERNAL: internal server error.
func (k *Karajo) apiAuthLogin(epr *libhttp.EndpointRequest) (respBody []byte, err error) {
	var (
		logp = `apiAuthLogin`
		name = epr.HttpRequest.Form.Get(paramNameName)
		pass = epr.HttpRequest.Form.Get(paramNamePassword)
	)

	name = strings.TrimSpace(name)
	if len(name) == 0 {
		return nil, &errAuthLogin
	}

	pass = strings.TrimSpace(pass)
	if len(pass) == 0 {
		return nil, &errAuthLogin
	}

	var user = k.env.Users[name]
	if user == nil {
		return nil, &errAuthLogin
	}

	if !user.authenticate(pass) {
		return nil, &errAuthLogin
	}

	err = k.sessionNew(epr.HttpWriter, user)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}

	var res = &libhttp.EndpointResponse{}

	res.Code = http.StatusOK
	respBody, err = json.Marshal(res)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}

	return respBody, nil
}

func (k *Karajo) apiEnvironment(epr *libhttp.EndpointRequest) (resbody []byte, err error) {
	var (
		logp = `apiEnvironment`
		res  = &libhttp.EndpointResponse{}
	)

	res.Code = http.StatusOK
	res.Data = k.env

	k.env.lockAllJob()
	resbody, err = json.Marshal(res)
	k.env.unlockAllJob()

	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}

	resbody, err = compressGzip(resbody)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}

	epr.HttpWriter.Header().Set(libhttp.HeaderContentEncoding, libhttp.ContentEncodingGzip)

	return resbody, nil
}

// apiJobLog get the job log by its ID and counter.
//
// # Request
//
// Format,
//
//	GET /karajo/api/job/log?id=<jobID>&counter=<counter>
//
// # Response
//
// If the jobID and counter exist it will return the JobLog object as JSON.
func (k *Karajo) apiJobLog(epr *libhttp.EndpointRequest) (resbody []byte, err error) {
	var (
		logp              = `apiJobLog`
		res               = &libhttp.EndpointResponse{}
		id         string = epr.HttpRequest.Form.Get(paramNameID)
		counterStr string = epr.HttpRequest.Form.Get(paramNameCounter)

		buf     bytes.Buffer
		job     *Job
		jlog    *JobLog
		counter int64
	)

	id = strings.ToLower(id)
	job = k.env.job(id)
	if job == nil {
		res.Code = http.StatusNotFound
		res.Message = fmt.Sprintf(`job ID %s not found`, id)
		return nil, res
	}

	counter, err = strconv.ParseInt(counterStr, 10, 64)
	if err != nil {
		res.Code = http.StatusNotFound
		res.Message = fmt.Sprintf(`log #%s not found`, counterStr)
		return nil, res
	}

	jlog = job.JobBase.log(counter)
	if jlog == nil {
		res.Code = http.StatusNotFound
		res.Message = fmt.Sprintf(`log #%s not found`, counterStr)
		goto out
	}

	err = jlog.load()
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}

	resbody, err = jlog.marshalJSON()
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}

	fmt.Fprintf(&buf, `{"code":200,"data":%s}`, resbody)

	resbody, err = compressGzip(buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}

	epr.HttpWriter.Header().Set(libhttp.HeaderContentEncoding, libhttp.ContentEncodingGzip)
	return resbody, nil

out:
	resbody, err = json.Marshal(res)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}
	return resbody, nil
}

// apiJobPause pause the Job.
//
// Request format,
//
//	POST /karajo/api/job/pause
//	Content-Type: application/x-www-form-urlencoded
//
//	_karajo_epoch=&id=
//
// List of response,
//
//   - 200: OK, if job ID is valid.
//   - 404: If job ID not found.
func (k *Karajo) apiJobPause(epr *libhttp.EndpointRequest) (resb []byte, err error) {
	var (
		logp = `apiJobPause`

		res *libhttp.EndpointResponse
		job *Job
		id  string
	)

	err = k.httpAuthorize(epr, epr.RequestBody)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}

	id = epr.HttpRequest.Form.Get(paramNameID)

	job = k.env.job(id)
	if job == nil {
		return nil, fmt.Errorf(`%s: %w`, logp, errJobNotFound(id))
	}

	job.pause()

	res = &libhttp.EndpointResponse{}
	res.Code = http.StatusOK
	res.Data = job

	job.Lock()
	resb, err = json.Marshal(res)
	job.Unlock()

	return resb, err
}

// apiJobResume resume the paused Job.
//
// # Request
//
//	POST /karajo/api/job/resume
//	Content-Type: application/x-www-form-urlencoded
//
//	_karajo_epoch=&id=
//
// # Response
//
//   - 200: OK, if job ID is valid.
//   - 404: If job ID not found.
func (k *Karajo) apiJobResume(epr *libhttp.EndpointRequest) (resb []byte, err error) {
	var (
		logp = `apiJobResume`

		res *libhttp.EndpointResponse
		job *Job
		id  string
	)

	err = k.httpAuthorize(epr, epr.RequestBody)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}

	id = epr.HttpRequest.Form.Get(paramNameID)

	job = k.env.job(id)
	if job == nil {
		return nil, fmt.Errorf(`%s: %w`, logp, errJobNotFound(id))
	}

	job.resume(``)

	job.Lock()
	defer job.Unlock()

	res = &libhttp.EndpointResponse{}
	res.Code = http.StatusOK
	res.Data = job

	return json.Marshal(res)
}

// apiJobHttp HTTP API to get the JobHttp information by its ID.
func (k *Karajo) apiJobHttp(epr *libhttp.EndpointRequest) (resbody []byte, err error) {
	var (
		res              = &libhttp.EndpointResponse{}
		id      string   = epr.HttpRequest.Form.Get(paramNameID)
		jobHttp *JobHttp = k.env.jobHttp(id)
	)

	if jobHttp == nil {
		return nil, errInvalidJobID(id)
	}

	res.Code = http.StatusOK
	res.Data = jobHttp

	jobHttp.Lock()
	resbody, err = json.Marshal(res)
	jobHttp.Unlock()

	return resbody, err
}

// apiJobHttpLog HTTP API to get the logs for JobHttp by its ID.
//
// Request format,
//
//	GET /karajo/api/job_http/log?id=<jobID>&counter=<counter>
//
// If the jobID and counter exist it will return the JobLog object as JSON.
func (k *Karajo) apiJobHttpLog(epr *libhttp.EndpointRequest) (resbody []byte, err error) {
	var (
		logp              = `apiJobHttpLog`
		res               = &libhttp.EndpointResponse{}
		id         string = epr.HttpRequest.Form.Get(paramNameID)
		counterStr string = epr.HttpRequest.Form.Get(paramNameCounter)

		buf     bytes.Buffer
		job     *JobHttp
		hlog    *JobLog
		counter int64
	)

	id = strings.ToLower(id)
	job = k.env.jobHttp(id)
	if job == nil {
		return nil, errInvalidJobID(id)
	}

	counter, err = strconv.ParseInt(counterStr, 10, 64)
	if err != nil {
		res.Code = http.StatusNotFound
		res.Message = fmt.Sprintf(`log #%s not found`, counterStr)
		return nil, res
	}

	jlog = job.JobBase.log(counter)
	if jlog == nil {
		res.Code = http.StatusNotFound
		res.Message = fmt.Sprintf(`log #%s not found`, counterStr)
		goto out
	}

	err = jlog.load()
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}

	resbody, err = jlog.marshalJSON()
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}

	fmt.Fprintf(&buf, `{"code":200,"data":%s}`, resbody)

	resbody, err = compressGzip(buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}

	epr.HttpWriter.Header().Set(libhttp.HeaderContentEncoding, libhttp.ContentEncodingGzip)
	return resbody, nil

out:
	resbody, err = json.Marshal(res)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}
	return resbody, nil
}

// apiJobHttpPause HTTP API to pause running the JobHttp.
func (k *Karajo) apiJobHttpPause(epr *libhttp.EndpointRequest) (resb []byte, err error) {
	var (
		res = &libhttp.EndpointResponse{}

		id      string
		jobHttp *JobHttp
	)

	err = k.httpAuthorize(epr, []byte(epr.HttpRequest.URL.RawQuery))
	if err != nil {
		return nil, err
	}

	id = epr.HttpRequest.Form.Get(paramNameID)
	jobHttp = k.env.jobHttp(id)
	if jobHttp == nil {
		return nil, errInvalidJobID(id)
	}

	jobHttp.pause()

	res.Code = http.StatusOK
	res.Data = jobHttp

	return json.Marshal(res)
}

// apiJobHttpResume HTTP API to resume running JobHttp.
func (k *Karajo) apiJobHttpResume(epr *libhttp.EndpointRequest) (resb []byte, err error) {
	var (
		res = &libhttp.EndpointResponse{}

		id      string
		jobHttp *JobHttp
	)

	err = k.httpAuthorize(epr, []byte(epr.HttpRequest.URL.RawQuery))
	if err != nil {
		return nil, err
	}

	id = epr.HttpRequest.Form.Get(paramNameID)
	jobHttp = k.env.jobHttp(id)
	if jobHttp == nil {
		return nil, errInvalidJobID(id)
	}

	jobHttp.resume(JobStatusStarted)

	res.Code = http.StatusOK
	res.Data = jobHttp

	return json.Marshal(res)
}

// httpAuthorize authorize request by checking the signature.
func (k *Karajo) httpAuthorize(epr *libhttp.EndpointRequest, payload []byte) (err error) {
	var (
		gotSign string
		expSign string
	)

	gotSign = epr.HttpRequest.Header.Get(HeaderNameXKarajoSign)
	if len(gotSign) == 0 {
		return &errUnauthorized
	}

	expSign = Sign(payload, k.env.secretb)
	if expSign != gotSign {
		return &errUnauthorized
	}

	return nil
}

func compressGzip(in []byte) (out []byte, err error) {
	var (
		logp  = `compressGzip`
		bufgz = bytes.Buffer{}
		gzw   = gzip.NewWriter(&bufgz)
	)

	_, err = gzw.Write(in)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}

	err = gzw.Close()
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}

	out = bufgz.Bytes()

	return out, nil
}
