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

	libhttp "github.com/shuLhan/share/lib/http"
	"github.com/shuLhan/share/lib/memfs"
)

// HeaderNameXKarajoSign the header key for the signature of body.
const HeaderNameXKarajoSign = `X-Karajo-Sign`

// List of HTTP API.
const (
	apiAuthLogin = `/karajo/api/auth/login`

	apiEnv = `/karajo/api/environment`

	apiJobHTTP       = `/karajo/api/job_http`
	apiJobHTTPLog    = `/karajo/api/job_http/log`
	apiJobHTTPPause  = `/karajo/api/job_http/pause`
	apiJobHTTPResume = `/karajo/api/job_http/resume`

	apiJobExecLog    = `/karajo/api/job_exec/log`
	apiJobExecPause  = `/karajo/api/job_exec/pause`
	apiJobExecResume = `/karajo/api/job_exec/resume`
	apiJobExecRun    = `/karajo/api/job_exec/run`
)

// List of known pathes.
const (
	pathKarajoAPI = `/karajo/api/`
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

// initHTTPd initialize the HTTP server, including registering its endpoints
// and JobExec endpoints.
func (k *Karajo) initHTTPd() (err error) {
	var (
		logp       = `initHTTPd`
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

	k.HTTPd, err = libhttp.NewServer(&serverOpts)
	if err != nil {
		return fmt.Errorf(`%s: %w`, logp, err)
	}

	err = k.registerAPIs()
	if err != nil {
		return fmt.Errorf(`%s: %w`, logp, err)
	}

	err = k.registerJobsHook()
	if err != nil {
		return fmt.Errorf(`%s: %w`, logp, err)
	}

	return nil
}

// registerAPIs register the public HTTP APIs.
func (k *Karajo) registerAPIs() (err error) {
	var logp = `registerAPIs`

	err = k.HTTPd.RegisterEndpoint(&libhttp.Endpoint{
		Method:       libhttp.RequestMethodPost,
		Path:         apiAuthLogin,
		RequestType:  libhttp.RequestTypeForm,
		ResponseType: libhttp.ResponseTypeJSON,
		Call:         k.apiAuthLogin,
	})
	if err != nil {
		return fmt.Errorf(`%s: %w`, logp, err)
	}

	err = k.HTTPd.RegisterEndpoint(&libhttp.Endpoint{
		Method:       libhttp.RequestMethodGet,
		Path:         apiEnv,
		RequestType:  libhttp.RequestTypeNone,
		ResponseType: libhttp.ResponseTypeJSON,
		Call:         k.apiEnv,
	})
	if err != nil {
		return err
	}

	err = k.HTTPd.RegisterEndpoint(&libhttp.Endpoint{
		Method:       libhttp.RequestMethodGet,
		Path:         apiJobExecLog,
		RequestType:  libhttp.RequestTypeQuery,
		ResponseType: libhttp.ResponseTypeJSON,
		Call:         k.apiJobExecLog,
	})
	if err != nil {
		return err
	}
	err = k.HTTPd.RegisterEndpoint(&libhttp.Endpoint{
		Method:       libhttp.RequestMethodPost,
		Path:         apiJobExecPause,
		RequestType:  libhttp.RequestTypeForm,
		ResponseType: libhttp.ResponseTypeJSON,
		Call:         k.apiJobExecPause,
	})
	if err != nil {
		return fmt.Errorf(`%s: %s: %w`, logp, apiJobExecPause, err)
	}
	err = k.HTTPd.RegisterEndpoint(&libhttp.Endpoint{
		Method:       libhttp.RequestMethodPost,
		Path:         apiJobExecResume,
		RequestType:  libhttp.RequestTypeForm,
		ResponseType: libhttp.ResponseTypeJSON,
		Call:         k.apiJobExecResume,
	})
	if err != nil {
		return fmt.Errorf(`%s: %s: %w`, logp, apiJobExecResume, err)
	}

	err = k.HTTPd.RegisterEndpoint(&libhttp.Endpoint{
		Method:       libhttp.RequestMethodGet,
		Path:         apiJobHTTP,
		RequestType:  libhttp.RequestTypeQuery,
		ResponseType: libhttp.ResponseTypeJSON,
		Call:         k.apiJobHTTP,
	})
	if err != nil {
		return err
	}
	err = k.HTTPd.RegisterEndpoint(&libhttp.Endpoint{
		Method:       libhttp.RequestMethodGet,
		Path:         apiJobHTTPLog,
		RequestType:  libhttp.RequestTypeQuery,
		ResponseType: libhttp.ResponseTypeJSON,
		Call:         k.apiJobHTTPLog,
	})
	if err != nil {
		return err
	}
	err = k.HTTPd.RegisterEndpoint(&libhttp.Endpoint{
		Method:       libhttp.RequestMethodPost,
		Path:         apiJobHTTPPause,
		RequestType:  libhttp.RequestTypeQuery,
		ResponseType: libhttp.ResponseTypeJSON,
		Call:         k.apiJobHTTPPause,
	})
	if err != nil {
		return err
	}
	err = k.HTTPd.RegisterEndpoint(&libhttp.Endpoint{
		Method:       libhttp.RequestMethodPost,
		Path:         apiJobHTTPResume,
		RequestType:  libhttp.RequestTypeQuery,
		ResponseType: libhttp.ResponseTypeJSON,
		Call:         k.apiJobHTTPResume,
	})
	if err != nil {
		return err
	}

	return nil
}

// registerJobsHook register endpoint for executing JobExec using HTTP POST.
func (k *Karajo) registerJobsHook() (err error) {
	var job *JobExec

	for _, job = range k.env.ExecJobs {
		if len(job.Path) == 0 {
			// Ignore any job that does not have path.
			continue
		}

		err = k.HTTPd.RegisterEndpoint(&libhttp.Endpoint{
			Method:       libhttp.RequestMethodPost,
			Path:         path.Join(apiJobExecRun, job.Path),
			RequestType:  libhttp.RequestTypeJSON,
			ResponseType: libhttp.ResponseTypeJSON,
			Call:         job.handleHTTP,
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
	if strings.HasPrefix(path, pathKarajoAPI) {
		return true
	}
	return false
}

func isLoginPage(path string) bool {
	return path == `/karajo` || path == `/karajo/` || path == `/karajo/index.html`
}

// unauthorized write HTTP status 401 Unauthorized and return false.
func (k *Karajo) unauthorized(w http.ResponseWriter, _ *http.Request) bool {
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

func (k *Karajo) apiEnv(epr *libhttp.EndpointRequest) (resbody []byte, err error) {
	var (
		logp = `apiEnv`
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

// apiJobExecLog get the JobExec log by its ID and counter.
//
// # Request
//
// Format,
//
//	GET /karajo/api/job_exec/log?id=<jobID>&counter=<counter>
//
// # Response
//
// If the jobID and counter exist it will return the JobLog object as JSON.
func (k *Karajo) apiJobExecLog(epr *libhttp.EndpointRequest) (resbody []byte, err error) {
	var (
		logp       = `apiJobExecLog`
		res        = &libhttp.EndpointResponse{}
		id         = epr.HttpRequest.Form.Get(paramNameID)
		counterStr = epr.HttpRequest.Form.Get(paramNameCounter)

		buf     bytes.Buffer
		job     *JobExec
		jlog    *JobLog
		counter int64
	)

	id = strings.ToLower(id)
	job = k.env.jobExec(id)
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

	jlog = job.JobBase.getLog(counter)
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

// apiJobExecPause pause the JobExec.
//
// Request format,
//
//	POST /karajo/api/job_exec/pause
//	Content-Type: application/x-www-form-urlencoded
//
//	_karajo_epoch=&id=
//
// List of response,
//
//   - 200: OK, if job ID is valid.
//   - 404: If job ID not found.
func (k *Karajo) apiJobExecPause(epr *libhttp.EndpointRequest) (resb []byte, err error) {
	var (
		logp = `apiJobExecPause`

		res *libhttp.EndpointResponse
		job *JobExec
		id  string
	)

	err = k.httpAuthorize(epr, epr.RequestBody)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}

	id = epr.HttpRequest.Form.Get(paramNameID)

	job = k.env.jobExec(id)
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

// apiJobExecResume resume the paused JobExec.
//
// # Request
//
//	POST /karajo/api/job_exec/resume
//	Content-Type: application/x-www-form-urlencoded
//
//	_karajo_epoch=&id=
//
// # Response
//
//   - 200: OK, if job ID is valid.
//   - 404: If job ID not found.
func (k *Karajo) apiJobExecResume(epr *libhttp.EndpointRequest) (resb []byte, err error) {
	var (
		logp = `apiJobExecResume`

		res *libhttp.EndpointResponse
		job *JobExec
		id  string
	)

	err = k.httpAuthorize(epr, epr.RequestBody)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}

	id = epr.HttpRequest.Form.Get(paramNameID)

	job = k.env.jobExec(id)
	if job == nil {
		return nil, fmt.Errorf(`%s: %w`, logp, errJobNotFound(id))
	}

	job.resume(``)

	res = &libhttp.EndpointResponse{}
	res.Code = http.StatusOK
	res.Data = job

	job.Lock()
	resb, err = json.Marshal(res)
	job.Unlock()

	return resb, err
}

// apiJobHTTP HTTP API to get the JobHTTP information by its ID.
func (k *Karajo) apiJobHTTP(epr *libhttp.EndpointRequest) (resbody []byte, err error) {
	var (
		res     = &libhttp.EndpointResponse{}
		id      = epr.HttpRequest.Form.Get(paramNameID)
		jobHTTP = k.env.jobHTTP(id)
	)

	if jobHTTP == nil {
		return nil, errInvalidJobID(id)
	}

	res.Code = http.StatusOK
	res.Data = jobHTTP

	jobHTTP.Lock()
	resbody, err = json.Marshal(res)
	jobHTTP.Unlock()

	return resbody, err
}

// apiJobHTTPLog HTTP API to get the logs for JobHTTP by its ID.
//
// Request format,
//
//	GET /karajo/api/job_http/log?id=<jobID>&counter=<counter>
//
// If the jobID and counter exist it will return the JobLog object as JSON.
func (k *Karajo) apiJobHTTPLog(epr *libhttp.EndpointRequest) (resbody []byte, err error) {
	var (
		logp       = `apiJobHTTPLog`
		res        = &libhttp.EndpointResponse{}
		id         = epr.HttpRequest.Form.Get(paramNameID)
		counterStr = epr.HttpRequest.Form.Get(paramNameCounter)

		buf     bytes.Buffer
		job     *JobHTTP
		jlog    *JobLog
		counter int64
	)

	id = strings.ToLower(id)
	job = k.env.jobHTTP(id)
	if job == nil {
		return nil, errInvalidJobID(id)
	}

	counter, err = strconv.ParseInt(counterStr, 10, 64)
	if err != nil {
		res.Code = http.StatusNotFound
		res.Message = fmt.Sprintf(`log #%s not found`, counterStr)
		return nil, res
	}

	jlog = job.JobBase.getLog(counter)
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

// apiJobHTTPPause HTTP API to pause running the JobHTTP.
func (k *Karajo) apiJobHTTPPause(epr *libhttp.EndpointRequest) (resb []byte, err error) {
	var (
		res = &libhttp.EndpointResponse{}

		id      string
		jobHTTP *JobHTTP
	)

	err = k.httpAuthorize(epr, []byte(epr.HttpRequest.URL.RawQuery))
	if err != nil {
		return nil, err
	}

	id = epr.HttpRequest.Form.Get(paramNameID)
	jobHTTP = k.env.jobHTTP(id)
	if jobHTTP == nil {
		return nil, errInvalidJobID(id)
	}

	jobHTTP.pause()

	res.Code = http.StatusOK
	res.Data = jobHTTP

	return json.Marshal(res)
}

// apiJobHTTPResume HTTP API to resume running JobHTTP.
func (k *Karajo) apiJobHTTPResume(epr *libhttp.EndpointRequest) (resb []byte, err error) {
	var (
		res = &libhttp.EndpointResponse{}

		id      string
		jobHTTP *JobHTTP
	)

	err = k.httpAuthorize(epr, []byte(epr.HttpRequest.URL.RawQuery))
	if err != nil {
		return nil, err
	}

	id = epr.HttpRequest.Form.Get(paramNameID)
	jobHTTP = k.env.jobHTTP(id)
	if jobHTTP == nil {
		return nil, errInvalidJobID(id)
	}

	jobHTTP.resume(JobStatusStarted)

	res.Code = http.StatusOK
	res.Data = jobHTTP

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
