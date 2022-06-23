// SPDX-FileCopyrightText: 2021 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

package karajo

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/shuLhan/share/lib/ini"
)

const (
	defEnvName       = "karajo"
	defListenAddress = ":31937"
	defHttpTimeout   = 5 * time.Minute
)

// Environment contains configuration for HTTP server, logs, and list of jobs.
type Environment struct {
	// List of Job indexed by ID.
	jobs map[string]*Job

	// Name of the service.
	// The Name will be used for title on the web user interface, as log
	// prefix, for file prefix on the jobs state, and as file prefix on
	// log files.
	// If this value is empty, it will be set to "karajo".
	Name string `ini:"karajo::name"`
	name string

	ListenAddress string `ini:"karajo::listen_address"`

	// DirBase define the base directory where configuration, job state,
	// and log stored.
	// This field is optional, default to current directory.
	// The structure of directory follow the UNIX system,
	//
	//	$DirBase
	//	|
	//	+-- /etc/karajo/karajo.conf
	//	|
	//	+-- /var/log/karajo/job/$Job.ID
	//      |
	//	+-- /var/run/karajo/job/$Job.ID
	//
	// Each job log stored under directory /var/log/karajo/job and the job
	// state under directory /var/run/karajo/job.
	DirBase    string `ini:"karajo::dir_base"`
	dirConfig  string
	dirLogJob  string
	dirRunJob  string
	dirCurrent string // The current directory where program running.

	file string

	// List of registered Job.
	Jobs []*Job `ini:"karajo:job"`

	// HttpTimeout define the default HTTP client timeout when executing
	// each jobs.
	// This field is optional, default to 5 minutes.
	HttpTimeout time.Duration `ini:"karajo::http_timeout"`

	// IsDevelopment if its true, the assets will be loaded directly from
	// disk instead from memory (memfs).
	IsDevelopment bool
}

// LoadEnvironment load the configuration from the ini file format.
func LoadEnvironment(file string) (env *Environment, err error) {
	var (
		logp = "LoadEnvironment"
		cfg  *ini.Ini
	)

	cfg, err = ini.Open(file)
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
	var (
		logp = "init"

		job *Job
	)

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

	err = env.initDirs()
	if err != nil {
		return fmt.Errorf("%s: %w", logp, err)
	}

	env.jobs = make(map[string]*Job, len(env.Jobs))
	for _, job = range env.Jobs {
		err = job.init(env)
		if err != nil {
			return fmt.Errorf("%s: %w", logp, err)
		}

		env.jobs[job.ID] = job
	}

	return nil
}

// initDirs create all job and log directories.
func (env *Environment) initDirs() (err error) {
	env.dirCurrent, err = os.Getwd()
	if err != nil {
		return err
	}

	if len(env.DirBase) == 0 {
		env.DirBase = env.dirCurrent
	}

	env.dirConfig = filepath.Join(env.DirBase, "etc", defEnvName)
	env.dirLogJob = filepath.Join(env.DirBase, "var", "log", defEnvName, "job")
	env.dirRunJob = filepath.Join(env.DirBase, "var", "run", defEnvName, "job")

	err = os.MkdirAll(env.dirLogJob, 0700)
	if err != nil {
		return fmt.Errorf("%s: %w", env.dirLogJob, err)
	}
	err = os.MkdirAll(env.dirRunJob, 0700)
	if err != nil {
		return fmt.Errorf("%s: %w", env.dirRunJob, err)
	}
	return nil
}

// jobsLock lock all the jobs.
func (env *Environment) jobsLock() {
	var job *Job
	for _, job = range env.jobs {
		job.Lock()
	}
}

func (env *Environment) jobsSave() (err error) {
	var job *Job
	for _, job = range env.jobs {
		err = job.stateSave()
		if err != nil {
			return err
		}
	}
	return nil
}

// jobsUnlock unlock all the jobs.
func (env *Environment) jobsUnlock() {
	var job *Job
	for _, job = range env.jobs {
		job.Unlock()
	}
}
