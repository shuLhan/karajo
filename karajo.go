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

const (
	Version = "0.5.0"

	// HeaderNameXKarajoSign define the header key for the signature of
	// body.
	HeaderNameXKarajoSign = "x-karajo-sign"

	apiEnvironment = "/karajo/api/environment"
	apiJob         = "/karajo/api/job"
	apiJobLogs     = "/karajo/api/job/logs"
	apiJobPause    = "/karajo/api/job/pause"
	apiJobResume   = "/karajo/api/job/resume"

	apiHook    = "/karajo/hook"
	apiHookLog = "/karajo/api/hook/log"

	paramNameID      = "id"
	paramNameCounter = "counter"
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
		Message: "empty or invalid signature",
	}

	// hookq is the channel that limit the number of hook running at the
	// same time.
	hookq chan struct{}
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
			Root: "_www",
			Excludes: []string{
				`.*\.adoc$`,
			},
			Embed: memfs.EmbedOptions{
				CommentHeader: `// SPDX-FileCopyrightText: 2021 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later
`,
				PackageName: "karajo",
				VarName:     "memfsWww",
				GoFileName:  "memfs_www.go",
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
		logp       = "New"
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
		return nil, fmt.Errorf("%s: %w", logp, err)
	}

	hookq = make(chan struct{}, env.MaxHookRunning)

	mlog.SetPrefix(env.Name + ":")

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
			return nil, fmt.Errorf("%s: %w", logp, err)
		}

		mfs = memfs.Merge(mfs, memfsWww)
		mfs.Root.SysPath = env.DirPublic
		mfs.Opts.TryDirect = true
		serverOpts.Memfs = mfs
	}

	k.Server, err = libhttp.NewServer(&serverOpts)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", logp, err)
	}

	err = k.registerApis()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", logp, err)
	}

	err = k.registerHooks()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", logp, err)
	}

	return k, nil
}

func (k *Karajo) registerApis() (err error) {
	err = k.RegisterEndpoint(&libhttp.Endpoint{
		Method:       libhttp.RequestMethodGet,
		Path:         apiEnvironment,
		RequestType:  libhttp.RequestTypeNone,
		ResponseType: libhttp.ResponseTypeJSON,
		Call:         k.apiEnvironment,
	})
	if err != nil {
		return err
	}

	err = k.RegisterEndpoint(&libhttp.Endpoint{
		Method:       libhttp.RequestMethodGet,
		Path:         apiHookLog,
		RequestType:  libhttp.RequestTypeQuery,
		ResponseType: libhttp.ResponseTypeJSON,
		Call:         k.apiHookLog,
	})
	if err != nil {
		return err
	}

	err = k.RegisterEndpoint(&libhttp.Endpoint{
		Method:       libhttp.RequestMethodGet,
		Path:         apiJob,
		RequestType:  libhttp.RequestTypeQuery,
		ResponseType: libhttp.ResponseTypeJSON,
		Call:         k.apiJob,
	})
	if err != nil {
		return err
	}
	err = k.RegisterEndpoint(&libhttp.Endpoint{
		Method:       libhttp.RequestMethodGet,
		Path:         apiJobLogs,
		RequestType:  libhttp.RequestTypeQuery,
		ResponseType: libhttp.ResponseTypeJSON,
		Call:         k.apiJobLogs,
	})
	if err != nil {
		return err
	}
	err = k.RegisterEndpoint(&libhttp.Endpoint{
		Method:       libhttp.RequestMethodPost,
		Path:         apiJobPause,
		RequestType:  libhttp.RequestTypeQuery,
		ResponseType: libhttp.ResponseTypeJSON,
		Call:         k.apiJobPause,
	})
	if err != nil {
		return err
	}
	err = k.RegisterEndpoint(&libhttp.Endpoint{
		Method:       libhttp.RequestMethodPost,
		Path:         apiJobResume,
		RequestType:  libhttp.RequestTypeQuery,
		ResponseType: libhttp.ResponseTypeJSON,
		Call:         k.apiJobResume,
	})
	if err != nil {
		return err
	}

	return nil
}

func (k *Karajo) registerHooks() (err error) {
	var hook *Hook

	for _, hook = range k.env.Hooks {
		err = k.RegisterEndpoint(&libhttp.Endpoint{
			Method:       libhttp.RequestMethodPost,
			Path:         path.Join(apiHook, hook.Path),
			RequestType:  libhttp.RequestTypeJSON,
			ResponseType: libhttp.ResponseTypeJSON,
			Call:         hook.run,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// Start all the jobs and the HTTP server.
func (k *Karajo) Start() (err error) {
	var jobHttp *JobHttp

	mlog.Outf("started the karajo server at http://%s/karajo", k.Server.Addr)

	for _, jobHttp = range k.env.httpJobs {
		go jobHttp.Start()
	}

	return k.Server.Start()
}

// Stop all the jobs and the HTTP server.
func (k *Karajo) Stop() (err error) {
	var jobHttp *JobHttp

	for _, jobHttp = range k.env.httpJobs {
		jobHttp.Stop()
	}

	err = k.env.jobsSave()
	if err != nil {
		mlog.Errf("Stop: %s", err)
	}

	return k.Server.Stop(5 * time.Second)
}

func (k *Karajo) apiEnvironment(epr *libhttp.EndpointRequest) (resbody []byte, err error) {
	var res = &libhttp.EndpointResponse{}
	res.Code = http.StatusOK
	res.Data = k.env

	k.env.jobsLock()
	k.env.hooksLock()
	resbody, err = json.Marshal(res)
	k.env.hooksUnlock()
	k.env.jobsUnlock()

	return resbody, err
}

// apiHookLog get the hook log by its ID and counter.
//
// # Request
//
// Format,
//
//	GET /karajo/hook/log?id=<hookID>&counter=<counter>
//
// # Response
//
// If the hookID and counter exist it will return the HookLog object as JSON.
func (k *Karajo) apiHookLog(epr *libhttp.EndpointRequest) (resbody []byte, err error) {
	var (
		res               = &libhttp.EndpointResponse{}
		id         string = epr.HttpRequest.Form.Get(paramNameID)
		counterStr string = epr.HttpRequest.Form.Get(paramNameCounter)

		hook    *Hook
		hlog    *HookLog
		counter int64
	)

	id = strings.ToLower(id)
	for _, hook = range k.env.Hooks {
		if hook.ID == id {
			break
		}
	}
	if hook == nil {
		res.Code = http.StatusNotFound
		res.Message = fmt.Sprintf("hook id %s not found", id)
		return nil, res
	}

	counter, err = strconv.ParseInt(counterStr, 10, 64)
	if err != nil {
		res.Code = http.StatusNotFound
		res.Message = fmt.Sprintf("log #%s not found", counterStr)
		return nil, res
	}

	for _, hlog = range hook.Logs {
		if hlog.Counter != counter {
			continue
		}
		break
	}

	if hlog == nil {
		res.Code = http.StatusNotFound
		res.Message = fmt.Sprintf("log #%s not found", counterStr)
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

// apiJob API to get job detail and its status.
// The api accept query parameter job "id".
func (k *Karajo) apiJob(epr *libhttp.EndpointRequest) (resbody []byte, err error) {
	var (
		res              = &libhttp.EndpointResponse{}
		id      string   = epr.HttpRequest.Form.Get(paramNameID)
		jobHttp *JobHttp = k.env.httpJobs[id]
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

func (k *Karajo) apiJobLogs(epr *libhttp.EndpointRequest) ([]byte, error) {
	var (
		res              = &libhttp.EndpointResponse{}
		id      string   = epr.HttpRequest.Form.Get(paramNameID)
		jobHttp *JobHttp = k.env.httpJobs[id]
	)

	if jobHttp == nil {
		return nil, errInvalidJobID(id)
	}

	res.Code = http.StatusOK
	res.Data = jobHttp.Log

	return json.Marshal(res)
}

// apiJobPause HTTP API to pause executing the job.
func (k *Karajo) apiJobPause(epr *libhttp.EndpointRequest) (resb []byte, err error) {
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
	jobHttp = k.env.httpJobs[id]
	if jobHttp == nil {
		return nil, errInvalidJobID(id)
	}

	jobHttp.pause()

	res.Code = http.StatusOK
	res.Data = jobHttp

	return json.Marshal(res)
}

// apiJobResume HTTP API to resume executing the job.
func (k *Karajo) apiJobResume(epr *libhttp.EndpointRequest) (resb []byte, err error) {
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
	jobHttp = k.env.httpJobs[id]
	if jobHttp == nil {
		return nil, errInvalidJobID(id)
	}

	jobHttp.resume()

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
