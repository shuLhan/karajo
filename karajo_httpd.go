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
)

// HeaderNameXKarajoSign the header key for the signature of body.
const HeaderNameXKarajoSign = `X-Karajo-Sign`

// List of HTTP API.
const (
	apiEnvironment = `/karajo/api/environment`

	apiJobHttp       = `/karajo/api/job_http`
	apiJobHttpLogs   = `/karajo/api/job_http/logs`
	apiJobHttpPause  = `/karajo/api/job_http/pause`
	apiJobHttpResume = `/karajo/api/job_http/resume`

	apiJobLog    = `/karajo/api/job/log`
	apiJobPause  = `/karajo/api/job/pause`
	apiJobResume = `/karajo/api/job/resume`
	apiJobRun    = `/karajo/api/job/run`
)

// List of known HTTP request parameters.
const (
	paramNameCounter     = `counter`
	paramNameID          = `id`
	paramNameKarajoEpoch = `_karajo_epoch`
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
		Path:         apiJobHttpLogs,
		RequestType:  libhttp.RequestTypeQuery,
		ResponseType: libhttp.ResponseTypeJSON,
		Call:         k.apiJobHttpLogs,
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

		job     *Job
		hlog    *JobLog
		counter int64
	)

	id = strings.ToLower(id)
	for _, job = range k.env.Jobs {
		if job.ID == id {
			break
		}
	}
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

	job.Lock()
	var v *JobLog
	for _, v = range job.Logs {
		if v.Counter != counter {
			continue
		}
		hlog = v
		break
	}
	job.Unlock()

	if hlog == nil {
		res.Code = http.StatusNotFound
		res.Message = fmt.Sprintf(`log #%s not found`, counterStr)
	} else {
		err = hlog.load()
		if err != nil {
			res.Code = http.StatusInternalServerError
			res.Message = err.Error()
		} else {
			res.Code = http.StatusOK
			res.Data = hlog
		}
	}

	resbody, err = json.Marshal(res)
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

// apiJobHttpLogs HTTP API to get the logs for JobHttp by its ID.
func (k *Karajo) apiJobHttpLogs(epr *libhttp.EndpointRequest) ([]byte, error) {
	var (
		res              = &libhttp.EndpointResponse{}
		id      string   = epr.HttpRequest.Form.Get(paramNameID)
		jobHttp *JobHttp = k.env.jobHttp(id)
	)

	if jobHttp == nil {
		return nil, errInvalidJobID(id)
	}

	res.Code = http.StatusOK
	res.Data = jobHttp.clog

	return json.Marshal(res)
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

	jobHttp.mlog.Outf(`pausing...`)
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

	jobHttp.mlog.Outf(`resuming...`)
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
