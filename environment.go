// SPDX-FileCopyrightText: 2021 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

package karajo

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/shuLhan/share/lib/ini"
	libhtml "github.com/shuLhan/share/lib/net/html"
)

const (
	defEnvName        = "karajo"
	defHttpTimeout    = 5 * time.Minute
	defListenAddress  = ":31937"
	defMaxHookRunning = 1
)

// Environment contains configuration for HTTP server, logs, and list of jobs.
type Environment struct {
	Hooks map[string]*Hook `ini:"hook"`

	// List of Job by name.
	HttpJobs map[string]*JobHttp `ini:"job.http"`
	httpJobs map[string]*JobHttp // List of Job indexed by ID.

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
	//	+-- /var/lib/karajo/hook/$Job.ID
	//	|
	//	+-- /var/log/karajo +-- /hook/$Hook.id
	//	|                   |
	//	|                   +-- /job/$Job.ID
	//      |
	//	+-- /var/run/karajo +-- /job/$Job.ID
	//
	// Each job log stored under directory /var/log/karajo/job and the job
	// state under directory /var/run/karajo/job.
	DirBase    string `ini:"karajo::dir_base"`
	dirConfig  string
	dirCurrent string // The current directory where program running.

	dirLibHook string

	dirLogHook string
	dirLogJob  string

	dirRunJob string

	file string

	// DirPublic define a path to serve to public.
	// While the WUI is served under "/karajo", a directory dir_public
	// will be served under "/".
	// A dir_public can contains sub directory as long as its name is not
	// "karajo".
	DirPublic string `ini:"karajo::dir_public"`

	// Secret contains string to authorize HTTP API using signature.
	// The signature is generated from HTTP payload (query or body) with
	// HMAC+SHA-256.
	// The signature is read from HTTP header "x-karajo-sign" as hex
	// string.
	// This field is optional, if its empty a random secret is generated
	// before server started and printed to stdout.
	Secret  string `ini:"karajo::secret" json:"-"`
	secretb []byte

	// HttpTimeout define the default HTTP client timeout when executing
	// each jobs.
	// This field is optional, default to 5 minutes.
	HttpTimeout time.Duration `ini:"karajo::http_timeout"`

	// MaxHookRunning define the maximum hook running at the same time.
	// This field is optional default to 1.
	MaxHookRunning int `ini:"karajo::max_hook_running"`

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

// ParseEnvironment parse the environment from raw bytes.
func ParseEnvironment(content []byte) (env *Environment, err error) {
	var (
		logp = `ParseEnvironment`
	)

	env = &Environment{}

	err = ini.Unmarshal(content, env)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}

	return env, nil
}

func (env *Environment) hooksLock() {
	var hook *Hook
	for _, hook = range env.Hooks {
		hook.Lock()
	}
}

func (env *Environment) hooksUnlock() {
	var hook *Hook
	for _, hook = range env.Hooks {
		hook.Unlock()
	}
}

func (env *Environment) init() (err error) {
	var (
		logp = "init"

		hook    *Hook
		jobHttp *JobHttp
		name    string
	)

	if len(env.Name) == 0 {
		env.Name = defEnvName
	}
	env.name = libhtml.NormalizeForID(env.Name)

	if len(env.ListenAddress) == 0 {
		env.ListenAddress = defListenAddress
	}
	if env.HttpTimeout == 0 {
		env.HttpTimeout = defHttpTimeout
	}
	if env.MaxHookRunning <= 0 {
		env.MaxHookRunning = defMaxHookRunning
	}

	if len(env.Secret) == 0 {
		return fmt.Errorf("%s: empty secret", logp)
	}
	env.secretb = []byte(env.Secret)

	err = env.initDirs()
	if err != nil {
		return fmt.Errorf("%s: %w", logp, err)
	}

	for name, hook = range env.Hooks {
		err = hook.init(env, name)
		if err != nil {
			return fmt.Errorf("%s: %w", logp, err)
		}
	}

	env.httpJobs = make(map[string]*JobHttp, len(env.HttpJobs))
	for name, jobHttp = range env.HttpJobs {
		err = jobHttp.init(env, name)
		if err != nil {
			return fmt.Errorf("%s: %w", logp, err)
		}
		env.httpJobs[jobHttp.ID] = jobHttp
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

	env.dirLibHook = filepath.Join(env.DirBase, "var", "lib", defEnvName, "hook")
	err = os.MkdirAll(env.dirLibHook, 0700)
	if err != nil {
		return fmt.Errorf("%s: %w", env.dirLibHook, err)
	}

	env.dirLogHook = filepath.Join(env.DirBase, "var", "log", defEnvName, "hook")
	err = os.MkdirAll(env.dirLogHook, 0700)
	if err != nil {
		return fmt.Errorf("%s: %w", env.dirLogHook, err)
	}

	env.dirLogJob = filepath.Join(env.DirBase, "var", "log", defEnvName, "job")
	err = os.MkdirAll(env.dirLogJob, 0700)
	if err != nil {
		return fmt.Errorf("%s: %w", env.dirLogJob, err)
	}

	env.dirRunJob = filepath.Join(env.DirBase, "var", "run", defEnvName, "job")
	err = os.MkdirAll(env.dirRunJob, 0700)
	if err != nil {
		return fmt.Errorf("%s: %w", env.dirRunJob, err)
	}

	return nil
}

// jobsLock lock all the jobs.
func (env *Environment) jobsLock() {
	var jobHttp *JobHttp
	for _, jobHttp = range env.httpJobs {
		jobHttp.Lock()
	}
}

func (env *Environment) jobsSave() (err error) {
	var jobHttp *JobHttp
	for _, jobHttp = range env.httpJobs {
		err = jobHttp.stateSave()
		if err != nil {
			return err
		}
	}
	return nil
}

// jobsUnlock unlock all the jobs.
func (env *Environment) jobsUnlock() {
	var jobHttp *JobHttp
	for _, jobHttp = range env.httpJobs {
		jobHttp.Unlock()
	}
}
