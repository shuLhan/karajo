// SPDX-FileCopyrightText: 2021 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

// Module karajo implement HTTP workers and manager similar to cron but works
// only on HTTP.
//
// karajo has the web user interface (WUI) for monitoring the jobs that run on
// port 31937 by default and can be configurable.
//
// A single instance of karajo is configured through code or configuration
// file using ini file format.
//
// For more information see the README file in this repository.
package karajo

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	liberrors "github.com/shuLhan/share/lib/errors"
	libhttp "github.com/shuLhan/share/lib/http"
	"github.com/shuLhan/share/lib/memfs"
	"github.com/shuLhan/share/lib/mlog"
)

// Version of this library and program.
const Version = `0.5.0`

// HeaderNameXKarajoSign the header key for the signature of body.
const HeaderNameXKarajoSign = `X-Karajo-Sign`

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

	paramNameCounter     = `counter`
	paramNameID          = `id`
	paramNameKarajoEpoch = `_karajo_epoch`
)

// TimeNow return the current time.
// It can be used in testing to provide static, predictable time.
var TimeNow = func() time.Time {
	return time.Now()
}

var (
	memfsWww *memfs.MemFS

	errUnauthorized = liberrors.E{
		Code:    http.StatusUnauthorized,
		Message: `empty or invalid signature`,
	}
)

type Karajo struct {
	*libhttp.Server

	env *Environment
}

// GenerateMemfs generate the memfs instance to start watching or embedding
// the _www directory.
func GenerateMemfs() (mfs *memfs.MemFS, err error) {
	var (
		opts = memfs.Options{
			Root: `_www`,
			Excludes: []string{
				`.*\.adoc$`,
			},
			Embed: memfs.EmbedOptions{
				CommentHeader: `// SPDX-FileCopyrightText: 2021 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later
`,
				PackageName: `karajo`,
				VarName:     `memfsWww`,
				GoFileName:  `memfs_www.go`,
			},
		}
	)

	mfs, err = memfs.New(&opts)
	if err != nil {
		return nil, err
	}

	return mfs, nil
}

// Sign generate hex string of HMAC + SHA256 of payload using the secret.
func Sign(payload, secret []byte) (sign string) {
	var (
		signer hash.Hash = hmac.New(sha256.New, secret)
		bsign  []byte
	)
	_, _ = signer.Write(payload)
	bsign = signer.Sum(nil)
	sign = hex.EncodeToString(bsign)
	return sign
}

// New create and initialize Karajo from configuration file.
func New(env *Environment) (k *Karajo, err error) {
	var (
		logp       = `New`
		serverOpts = libhttp.ServerOptions{
			Conn: &http.Server{
				ReadTimeout:    10 * time.Minute,
				WriteTimeout:   10 * time.Minute,
				MaxHeaderBytes: 1 << 20,
			},
			Memfs:           memfsWww,
			EnableIndexHtml: true,
		}
	)

	k = &Karajo{
		env: env,
	}

	err = env.init()
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}

	mlog.SetPrefix(env.Name + `:`)

	serverOpts.Address = k.env.ListenAddress

	if memfsWww == nil {
		memfsWww, err = GenerateMemfs()
		if err != nil {
			return nil, err
		}
	}

	memfsWww.Opts.TryDirect = env.IsDevelopment

	if len(env.DirPublic) != 0 {
		var (
			opts = memfs.Options{
				Root:      env.DirPublic,
				TryDirect: true,
			}
			mfs *memfs.MemFS
		)

		mfs, err = memfs.New(&opts)
		if err != nil {
			return nil, fmt.Errorf(`%s: %w`, logp, err)
		}

		mfs = memfs.Merge(mfs, memfsWww)
		mfs.Root.SysPath = env.DirPublic
		mfs.Opts.TryDirect = true
		serverOpts.Memfs = mfs
	}

	k.Server, err = libhttp.NewServer(&serverOpts)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}

	err = k.registerApis()
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}

	err = k.registerJobsHook()
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}

	return k, nil
}

func (k *Karajo) registerApis() (err error) {
	var (
		logp = `registerApis`
	)

	err = k.Server.RegisterEndpoint(&libhttp.Endpoint{
		Method:       libhttp.RequestMethodGet,
		Path:         apiEnvironment,
		RequestType:  libhttp.RequestTypeNone,
		ResponseType: libhttp.ResponseTypeJSON,
		Call:         k.apiEnvironment,
	})
	if err != nil {
		return err
	}

	err = k.Server.RegisterEndpoint(&libhttp.Endpoint{
		Method:       libhttp.RequestMethodGet,
		Path:         apiJobLog,
		RequestType:  libhttp.RequestTypeQuery,
		ResponseType: libhttp.ResponseTypeJSON,
		Call:         k.apiJobLog,
	})
	if err != nil {
		return err
	}
	err = k.Server.RegisterEndpoint(&libhttp.Endpoint{
		Method:       libhttp.RequestMethodPost,
		Path:         apiJobPause,
		RequestType:  libhttp.RequestTypeForm,
		ResponseType: libhttp.ResponseTypeJSON,
		Call:         k.apiJobPause,
	})
	if err != nil {
		return fmt.Errorf(`%s: %s: %w`, logp, apiJobPause, err)
	}
	err = k.Server.RegisterEndpoint(&libhttp.Endpoint{
		Method:       libhttp.RequestMethodPost,
		Path:         apiJobResume,
		RequestType:  libhttp.RequestTypeForm,
		ResponseType: libhttp.ResponseTypeJSON,
		Call:         k.apiJobResume,
	})
	if err != nil {
		return fmt.Errorf(`%s: %s: %w`, logp, apiJobResume, err)
	}

	err = k.Server.RegisterEndpoint(&libhttp.Endpoint{
		Method:       libhttp.RequestMethodGet,
		Path:         apiJobHttp,
		RequestType:  libhttp.RequestTypeQuery,
		ResponseType: libhttp.ResponseTypeJSON,
		Call:         k.apiJobHttp,
	})
	if err != nil {
		return err
	}
	err = k.Server.RegisterEndpoint(&libhttp.Endpoint{
		Method:       libhttp.RequestMethodGet,
		Path:         apiJobHttpLogs,
		RequestType:  libhttp.RequestTypeQuery,
		ResponseType: libhttp.ResponseTypeJSON,
		Call:         k.apiJobHttpLogs,
	})
	if err != nil {
		return err
	}
	err = k.Server.RegisterEndpoint(&libhttp.Endpoint{
		Method:       libhttp.RequestMethodPost,
		Path:         apiJobHttpPause,
		RequestType:  libhttp.RequestTypeQuery,
		ResponseType: libhttp.ResponseTypeJSON,
		Call:         k.apiJobHttpPause,
	})
	if err != nil {
		return err
	}
	err = k.Server.RegisterEndpoint(&libhttp.Endpoint{
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

		err = k.Server.RegisterEndpoint(&libhttp.Endpoint{
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

// Start all the jobs and the HTTP server.
func (k *Karajo) Start() (err error) {
	var (
		jobHttp *JobHttp
		job     *Job
	)

	mlog.Outf(`started the karajo server at http://%s/karajo`, k.Server.Addr)

	for _, job = range k.env.Jobs {
		go job.Start()
	}
	for _, jobHttp = range k.env.HttpJobs {
		go jobHttp.Start()
	}

	return k.Server.Start()
}

// Stop all the jobs and the HTTP server.
func (k *Karajo) Stop() (err error) {
	var (
		jobHttp *JobHttp
		job     *Job
	)

	for _, jobHttp = range k.env.HttpJobs {
		jobHttp.Stop()
	}
	err = k.env.httpJobsSave()
	if err != nil {
		mlog.Errf(`Stop: %s`, err)
	}

	for _, job = range k.env.Jobs {
		job.Stop()
	}

	return k.Server.Stop(5 * time.Second)
}

func (k *Karajo) apiEnvironment(epr *libhttp.EndpointRequest) (resbody []byte, err error) {
	var res = &libhttp.EndpointResponse{}
	res.Code = http.StatusOK
	res.Data = k.env

	k.env.jobsLock()
	k.env.httpJobsLock()
	resbody, err = json.Marshal(res)
	k.env.httpJobsUnlock()
	k.env.jobsUnlock()

	return resbody, err
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

	return json.Marshal(res)
}

// apiJobPause pause the Job.
//
// # Request
//
//	POST /karajo/api/job/pause
//	Content-Type: application/x-www-form-urlencoded
//
//	_karajo_epoch=&id=
//
// # Response
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

	job.Lock()
	defer job.Unlock()

	res = &libhttp.EndpointResponse{}
	res.Code = http.StatusOK
	res.Data = job

	return json.Marshal(res)
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
