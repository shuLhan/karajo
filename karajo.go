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
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	libhttp "github.com/shuLhan/share/lib/http"
	"github.com/shuLhan/share/lib/memfs"
	"github.com/shuLhan/share/lib/mlog"
)

const (
	Version = "0.1.0"

	apiEnvironment = "/karajo/api/environment"
	apiJob         = "/karajo/api/job"
	apiJobLogs     = "/karajo/api/job/logs"
	apiJobPause    = "/karajo/api/job/pause/:id"
	apiJobResume   = "/karajo/api/job/resume/:id"

	apiTestJobFail    = "/karajo/test/job/fail"
	apiTestJobSuccess = "/karajo/test/job/success"

	paramNameID = "id"
)

var (
	memfsWww *memfs.MemFS
)

type Karajo struct {
	*libhttp.Server
	env *Environment
}

// New create and initialize Karajo from configuration file.
func New(env *Environment) (k *Karajo, err error) {
	var (
		logp       = "New"
		serverOpts = libhttp.ServerOptions{
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

	memfsWww.Opts.Development = env.IsDevelopment

	k.Server, err = libhttp.NewServer(&serverOpts)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", logp, err)
	}

	err = k.registerApis()
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
		RequestType:  libhttp.RequestTypeNone,
		ResponseType: libhttp.ResponseTypeJSON,
		Call:         k.apiJobPause,
	})
	if err != nil {
		return err
	}
	err = k.RegisterEndpoint(&libhttp.Endpoint{
		Method:       libhttp.RequestMethodPost,
		Path:         apiJobResume,
		RequestType:  libhttp.RequestTypeNone,
		ResponseType: libhttp.ResponseTypeJSON,
		Call:         k.apiJobResume,
	})
	if err != nil {
		return err
	}

	if k.env.IsDevelopment {
		// Endpoints for testing the jobs.
		err = k.RegisterEndpoint(&libhttp.Endpoint{
			Method:       libhttp.RequestMethodGet,
			Path:         apiTestJobFail,
			RequestType:  libhttp.RequestTypeQuery,
			ResponseType: libhttp.ResponseTypeJSON,
			Call:         k.apiTestJobFail,
		})
		if err != nil {
			return err
		}
		err = k.RegisterEndpoint(&libhttp.Endpoint{
			Method:       libhttp.RequestMethodGet,
			Path:         apiTestJobSuccess,
			RequestType:  libhttp.RequestTypeQuery,
			ResponseType: libhttp.ResponseTypeJSON,
			Call:         k.apiTestJobSuccess,
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
	res.Data = job.logs.Slice()

	return json.Marshal(res)
}

// apiJobPause HTTP API to pause executing the job.
func (k *Karajo) apiJobPause(epr *libhttp.EndpointRequest) ([]byte, error) {
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

	job.pause()

	res.Code = http.StatusOK
	res.Data = job

	return json.Marshal(res)
}

// apiJobResume HTTP API to resume executing the job.
func (k *Karajo) apiJobResume(epr *libhttp.EndpointRequest) ([]byte, error) {
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

	job.resume()

	res.Code = http.StatusOK
	res.Data = job

	return json.Marshal(res)
}

func (k *Karajo) apiTestJobFail(_ *libhttp.EndpointRequest) ([]byte, error) {
	var res = &libhttp.EndpointResponse{}
	res.Code = http.StatusBadRequest
	res.Message = "The job has failed"
	return nil, res
}

func (k *Karajo) apiTestJobSuccess(_ *libhttp.EndpointRequest) ([]byte, error) {
	var res = &libhttp.EndpointResponse{}
	res.Code = http.StatusOK
	res.Message = "The job has been run successfully"
	return json.Marshal(res)
}
