// Copyright 2021, Shulhan <ms@kilabit.info>. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

//
// Module karajo implement HTTP workers and manager similar to AppEngine
// cron.
//
// karajo has the web user interface (WUI) for monitoring the jobs that run on
// port 31937 by default and can be configurable.
//
// A single instance of karajo is configured through an Environment or loaded
// from ini file format.
// There are three configuration sections, one to configure the server, one to
// configure the logs, and another one to configure one or more jobs to be
// executed.
//
// For more information see the README file in this repository.
//
package karajo

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/shuLhan/share/lib/debug"
	libhttp "github.com/shuLhan/share/lib/http"
	"github.com/shuLhan/share/lib/memfs"
	"github.com/shuLhan/share/lib/mlog"
)

const (
	apiEnvironment = "/karajo/api/environment"
	apiJob         = "/karajo/api/job"
	apiJobLogs     = "/karajo/api/job/logs"

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

//
// New create and initialize Karajo from configuration file.
//
func New(env *Environment) (k *Karajo, err error) {
	k = &Karajo{
		env: env,
	}

	err = env.init()
	if err != nil {
		return nil, fmt.Errorf("New: %w", err)
	}

	serverOpts := libhttp.ServerOptions{
		Options: memfs.Options{
			Root:        "_www",
			Development: debug.Value >= 2,
		},
		Memfs:   memfsWww,
		Address: k.env.ListenAddress,
	}

	k.Server, err = libhttp.NewServer(&serverOpts)
	if err != nil {
		return nil, fmt.Errorf("New: %w", err)
	}

	err = k.registerApis()
	if err != nil {
		return nil, fmt.Errorf("New: %w", err)
	}

	return k, nil
}

func (k *Karajo) registerApis() (err error) {
	err = k.RegisterEndpoint(&libhttp.Endpoint{
		Method:       libhttp.RequestMethodGet,
		Path:         apiEnvironment,
		RequestType:  libhttp.RequestTypeNone,
		ResponseType: libhttp.ResponseTypeJSON,
		Call:         k.apiEnvironmentGet,
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

	if debug.Value >= 1 {
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

//
// Start all the jobs and the HTTP server.
//
func (k *Karajo) Start() (err error) {
	mlog.Outf("started the server at %s\n", k.Server.Addr)

	for _, job := range k.env.Jobs {
		go job.Start()
	}
	return k.Server.Start()
}

//
// Stop all the jobs and the HTTP server.
//
func (k *Karajo) Stop() (err error) {
	for _, job := range k.env.Jobs {
		job.Stop()
	}

	err = k.env.saveJobs()
	if err != nil {
		mlog.Errf("Stop: %s", err)
	}

	return k.Server.Stop(5 * time.Second)
}

func (k *Karajo) apiEnvironmentGet(epr *libhttp.EndpointRequest) ([]byte, error) {
	res := &libhttp.EndpointResponse{}
	res.Code = http.StatusOK
	res.Data = k.env
	return json.Marshal(res)
}

//
// apiJob API to get job detail and its status.
// The api accept query parameter job "id".
//
func (k *Karajo) apiJob(epr *libhttp.EndpointRequest) ([]byte, error) {
	res := &libhttp.EndpointResponse{}
	id := epr.HttpRequest.Form.Get(paramNameID)
	job := k.env.jobs[id]
	if job == nil {
		res.Code = http.StatusBadRequest
		res.Message = fmt.Sprintf("invalid or empty job id: %s", id)
		return nil, res
	}

	res.Code = http.StatusOK
	res.Data = job

	return json.Marshal(res)
}

func (k *Karajo) apiJobLogs(epr *libhttp.EndpointRequest) ([]byte, error) {
	res := &libhttp.EndpointResponse{}
	id := epr.HttpRequest.Form.Get(paramNameID)
	job := k.env.jobs[id]
	if job == nil {
		res.Code = http.StatusBadRequest
		res.Message = fmt.Sprintf("invalid or empty job id: %s", id)
		return nil, res
	}

	res.Code = http.StatusOK
	res.Data = job.logs.Slice()

	return json.Marshal(res)
}

func (k *Karajo) apiTestJobFail(_ *libhttp.EndpointRequest) ([]byte, error) {
	res := &libhttp.EndpointResponse{}
	res.Code = http.StatusBadRequest
	res.Message = "The job has failed"
	return nil, res
}

func (k *Karajo) apiTestJobSuccess(_ *libhttp.EndpointRequest) ([]byte, error) {
	res := &libhttp.EndpointResponse{}
	res.Code = http.StatusOK
	res.Message = "The job has been run successfully"
	return json.Marshal(res)
}
