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
	"path/filepath"
	"time"

	"github.com/shuLhan/share/lib/ini"
	"github.com/shuLhan/share/lib/mlog"
)

const (
	defEnvName           = "karajo"
	defListenAddress     = ":31937"
	defHttpTimeout       = 5 * time.Minute
	defFileLastRunSuffix = ".lastrun"

	envKarajoDevelopment = "KARAJO_DEVELOPMENT"
)

//
// Environment contains configuration for HTTP server, logs, and list of jobs.
//
type Environment struct {
	// Name of the service.
	// The Name will be used for title on the web user interface, as log
	// prefix, for file prefix on the jobs state, and as file prefix on
	// log files.
	// If this value is empty, it will be set to "karajo".
	Name string `ini:"karajo::name"`
	name string

	ListenAddress string `ini:"karajo::listen_address"`

	// HttpTimeout define the defaukt HTTP client timeout when executing
	// each jobs.
	// This field is optional, default to 5 minutes.
	HttpTimeout time.Duration `ini:"karajo::http_timeout"`

	// DirLogs contains path to the directory where log for each jobs will
	// be stored.
	// If this value is empty, all job logs will be written to stdout and
	// stderr.
	DirLogs string `ini:"karajo::dir_logs"`

	Jobs []*Job `ini:"karajo:job"`
	jobs map[string]*Job

	file        string
	fileLastRun string

	// isDevelopment will be true if environment variable
	// KARAJO_DEVELOPMENT is set to non-empty string.
	// If its true, the assets will be loaded directly from disk instead
	// from memory (memfs).
	isDevelopment bool
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
		file: file,
	}

	err = cfg.Unmarshal(env)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", logp, err)
	}

	return env, nil
}

func (env *Environment) init() (err error) {
	logp := "init"

	gob.Register(Job{})

	if len(env.Name) == 0 {
		env.Name = defEnvName
	}
	env.name = generateID(env.Name)

	if len(env.ListenAddress) == 0 {
		env.ListenAddress = defListenAddress
	}
	if env.HttpTimeout == 0 {
		env.HttpTimeout = defHttpTimeout
	}

	if len(env.file) > 0 {
		env.fileLastRun = env.file + defFileLastRunSuffix
	} else {
		wd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("%s: %w", logp, err)
		}
		env.fileLastRun = filepath.Join(wd, env.name+defFileLastRunSuffix)
	}

	prevJobs, err := env.loadJobs()
	if err != nil {
		mlog.Errf("%s: %s\n", logp, err)
	}

	env.jobs = make(map[string]*Job, len(env.Jobs))
	for _, job := range env.Jobs {
		err = job.init(env)
		if err != nil {
			return fmt.Errorf("%s: %w", logp, err)
		}

		env.jobs[job.ID] = job

		prevJob := prevJobs[job.ID]
		if prevJob != nil {
			job.LastRun = prevJob.LastRun
			job.LastStatus = prevJob.LastStatus
			job.IsPausing = prevJob.IsPausing
		}
	}

	env.isDevelopment = len(os.Getenv(envKarajoDevelopment)) > 0

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
