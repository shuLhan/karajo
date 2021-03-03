// Copyright 2021, Shulhan <ms@kilabit.info>. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package karajo

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/shuLhan/share/lib/ini"
)

const (
	defListenAddress     = ":31937"
	defHttpTimeout       = 5 * time.Minute
	defFileLastRunSuffix = ".lastrun"
)

//
// Environment contains configuration for HTTP server, logs, and list of jobs.
//
type Environment struct {
	ListenAddress string `ini:"karajo::listen_address"`

	// HttpTimeout define the HTTP timeout when executing each jobs.
	// This field is optional, default to 5 minutes.
	HttpTimeout time.Duration `ini:"karajo::http_timeout"`

	LogOptions LogOptions `ini:"karajo:log"`
	Jobs       []*Job     `ini:"karajo:job"`
	jobs       map[string]*Job

	file        string
	fileLastRun string
}

//
// LoadEnvironment load the configuration from the ini file format.
//
func LoadEnvironment(file string) (env *Environment, err error) {
	logp := "LoadEnvironment"

	cfg, err := ini.Open(file)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", logp, err)
	}

	env = &Environment{
		file:        file,
		fileLastRun: file + defFileLastRunSuffix,
	}

	err = cfg.Unmarshal(env)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", logp, err)
	}

	return env, nil
}

func (env *Environment) init() (err error) {
	gob.Register(Job{})

	if len(env.ListenAddress) == 0 {
		env.ListenAddress = defListenAddress
	}
	if env.HttpTimeout == 0 {
		env.HttpTimeout = defHttpTimeout
	}

	prevJobs, err := env.loadJobs()
	if err != nil {
		return fmt.Errorf("init: %w", err)
	}

	env.jobs = make(map[string]*Job, len(env.Jobs))
	for _, job := range env.Jobs {
		err = job.init(env.ListenAddress)
		if err != nil {
			return err
		}

		env.jobs[job.ID] = job

		prevJob := prevJobs[job.ID]
		if prevJob != nil {
			job.LastRun = prevJob.LastRun
		}
	}

	return nil
}

//
// loadJobs load previous saved job from file.
//
func (env *Environment) loadJobs() (lastJobs map[string]*Job, err error) {
	b, err := ioutil.ReadFile(env.fileLastRun)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("loadJobs: %w", err)
	}

	dec := gob.NewDecoder(bytes.NewReader(b))

	lastJobs = make(map[string]*Job)
	err = dec.Decode(&lastJobs)
	if err != nil {
		return nil, fmt.Errorf("loadJobs: %w", err)
	}

	return lastJobs, nil
}

//
// saveJobs save all the jobs data into file ending with ".lastrun".
//
func (env *Environment) saveJobs() (err error) {
	var buf bytes.Buffer

	enc := gob.NewEncoder(&buf)
	err = enc.Encode(&env.jobs)
	if err != nil {
		return fmt.Errorf("saveJobs: %w", err)
	}

	err = ioutil.WriteFile(env.fileLastRun, buf.Bytes(), 0600)
	if err != nil {
		return fmt.Errorf("saveJobs: %w", err)
	}

	return nil
}
