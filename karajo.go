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
	Version = "0.1.0"

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

var (
	memfsWww *memfs.MemFS

	errUnauthorized = liberrors.E{
		Code:    http.StatusUnauthorized,
		Message: "empty or invalid signature",
	}
)

type Karajo struct {
	*libhttp.Server
	env *Environment
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
			Memfs: memfsWww,
		}
	)

	k = &Karajo{
		env: env,
	}

	err = env.init()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", logp, err)
	}

	mlog.SetPrefix(env.Name + ":")

	serverOpts.Address = k.env.ListenAddress

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
	mlog.Outf("started the karajo server at http://%s/karajo\n", k.Server.Addr)

	var job *Job
	for _, job = range k.env.Jobs {
		go job.Start()
	}

	return k.Server.Start()
}

// Stop all the jobs and the HTTP server.
func (k *Karajo) Stop() (err error) {
	var job *Job

	for _, job = range k.env.Jobs {
		job.Stop()
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
	resbody, err = json.Marshal(res)
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
		if len(hlog.Content) == 0 {
			err = hlog.load()
			if err != nil {
				res.Code = http.StatusInternalServerError
				res.Message = err.Error()
				return nil, res
			}
		}

		res.Code = http.StatusOK
		res.Data = hlog
		return json.Marshal(res)
	}

	res.Code = http.StatusNotFound
	res.Message = fmt.Sprintf("log #%s not found", counterStr)

	return json.Marshal(res)
}

// apiJob API to get job detail and its status.
// The api accept query parameter job "id".
func (k *Karajo) apiJob(epr *libhttp.EndpointRequest) (resbody []byte, err error) {
	var (
		res        = &libhttp.EndpointResponse{}
		id  string = epr.HttpRequest.Form.Get(paramNameID)
		job *Job   = k.env.jobs[id]
	)

	if job == nil {
		res.Code = http.StatusBadRequest
		res.Message = fmt.Sprintf("invalid or empty job id: %s", id)
		return nil, res
	}

	res.Code = http.StatusOK
	res.Data = job

	job.Lock()
	resbody, err = json.Marshal(res)
	job.Unlock()

	return resbody, err
}

func (k *Karajo) apiJobLogs(epr *libhttp.EndpointRequest) ([]byte, error) {
	var (
		res        = &libhttp.EndpointResponse{}
		id  string = epr.HttpRequest.Form.Get(paramNameID)
		job *Job   = k.env.jobs[id]
	)

	if job == nil {
		res.Code = http.StatusBadRequest
		res.Message = fmt.Sprintf("invalid or empty job id: %s", id)
		return nil, res
	}

	res.Code = http.StatusOK
	res.Data = job.Log

	return json.Marshal(res)
}

// apiJobPause HTTP API to pause executing the job.
func (k *Karajo) apiJobPause(epr *libhttp.EndpointRequest) (resb []byte, err error) {
	var (
		res = &libhttp.EndpointResponse{}

		id  string
		job *Job
	)

	err = k.httpAuthorize(epr, []byte(epr.HttpRequest.URL.RawQuery))
	if err != nil {
		return nil, err
	}

	id = epr.HttpRequest.Form.Get(paramNameID)
	job = k.env.jobs[id]
	if job == nil {
		res.Code = http.StatusBadRequest
		res.Message = fmt.Sprintf("invalid or empty job id: %s", id)
		return nil, res
	}

	job.pause()

	res.Code = http.StatusOK
	res.Data = job

	return json.Marshal(res)
}

// apiJobResume HTTP API to resume executing the job.
func (k *Karajo) apiJobResume(epr *libhttp.EndpointRequest) (resb []byte, err error) {
	var (
		res = &libhttp.EndpointResponse{}

		id  string
		job *Job
	)

	err = k.httpAuthorize(epr, []byte(epr.HttpRequest.URL.RawQuery))
	if err != nil {
		return nil, err
	}

	id = epr.HttpRequest.Form.Get(paramNameID)
	job = k.env.jobs[id]
	if job == nil {
		res.Code = http.StatusBadRequest
		res.Message = fmt.Sprintf("invalid or empty job id: %s", id)
		return nil, res
	}

	job.resume()

	res.Code = http.StatusOK
	res.Data = job

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
