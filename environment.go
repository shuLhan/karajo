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
	defEnvName       = `karajo`
	defHttpTimeout   = 5 * time.Minute
	defListenAddress = `:31937`
	defMaxJobRunning = 1
)

// Environment contains configuration for HTTP server, logs, and list of jobs.
type Environment struct {
	Jobs map[string]*Job `ini:"job"`

	// jobq is the channel that limit the number of job running at the
	// same time.
	// This limit can be overwritten by MaxJobRunning.
	jobq chan struct{}

	// List of Job by name.
	HttpJobs map[string]*JobHttp `ini:"job.http"`

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
	//	+-- /var/lib/karajo/job/$Job.ID
	//	|
	//	+-- /var/log/karajo +-- /job/$Job.ID
	//	|                   |
	//	|                   +-- /job_http/$Job.ID
	//	|
	//	+-- /var/run/karajo/job_http/$Job.ID
	//
	// Each job log stored under directory /var/log/karajo/job and the job
	// state under directory /var/run/karajo/job.
	DirBase    string `ini:"karajo::dir_base"`
	dirConfig  string
	dirCurrent string // The current directory where program running.

	dirLibJob     string
	dirLogJob     string
	dirLogJobHttp string

	// dirRunJobHttp define the directory where JobHttp state is stored.
	dirRunJobHttp string

	file string

	// DirPublic define a path to serve to public.
	// While the WUI is served under "/karajo", a directory DirPublic
	// will be served under "/".
	// A DirPublic can contains sub directory as long as its name is not
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

	// HttpTimeout define the global HTTP client timeout when executing
	// each jobs.
	// This field is optional, default to 5 minutes.
	HttpTimeout time.Duration `ini:"karajo::http_timeout"`

	// MaxJobRunning define the maximum job running at the same time.
	// This field is optional default to 1.
	MaxJobRunning int `ini:"karajo::max_job_running"`

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

// job get the Job by its ID.
func (env *Environment) job(id string) (job *Job) {
	for _, job = range env.Jobs {
		if job.ID == id {
			return job
		}
	}
	return nil
}

// jobHttp get the registered JobHttp by its ID.
func (env *Environment) jobHttp(id string) (job *JobHttp) {
	for _, job = range env.HttpJobs {
		if job.ID == id {
			return job
		}
	}
	return nil
}

func (env *Environment) jobsLock() {
	var job *Job
	for _, job = range env.Jobs {
		job.Lock()
	}
}

func (env *Environment) jobsUnlock() {
	var job *Job
	for _, job = range env.Jobs {
		job.Unlock()
	}
}

func (env *Environment) init() (err error) {
	var (
		logp = "init"

		job     *Job
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
	if env.MaxJobRunning <= 0 {
		env.MaxJobRunning = defMaxJobRunning
	}
	env.jobq = make(chan struct{}, env.MaxJobRunning)

	if len(env.Secret) == 0 {
		return fmt.Errorf("%s: empty secret", logp)
	}
	env.secretb = []byte(env.Secret)

	err = env.initDirs()
	if err != nil {
		return fmt.Errorf("%s: %w", logp, err)
	}

	for name, job = range env.Jobs {
		err = job.init(env, name)
		if err != nil {
			return fmt.Errorf("%s: %w", logp, err)
		}
	}

	for name, jobHttp = range env.HttpJobs {
		err = jobHttp.init(env, name)
		if err != nil {
			return fmt.Errorf("%s: %w", logp, err)
		}
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

	env.dirLibJob = filepath.Join(env.DirBase, `var`, `lib`, defEnvName, `job`)
	err = os.MkdirAll(env.dirLibJob, 0700)
	if err != nil {
		return fmt.Errorf(`%s: %w`, env.dirLibJob, err)
	}

	env.dirLogJob = filepath.Join(env.DirBase, "var", "log", defEnvName, "job")
	err = os.MkdirAll(env.dirLogJob, 0700)
	if err != nil {
		return fmt.Errorf("%s: %w", env.dirLogJob, err)
	}

	env.dirLogJobHttp = filepath.Join(env.DirBase, `var`, `log`, defEnvName, `job_http`)
	err = os.MkdirAll(env.dirLogJobHttp, 0700)
	if err != nil {
		return fmt.Errorf(`%s: %w`, env.dirLogJobHttp, err)
	}

	env.dirRunJobHttp = filepath.Join(env.DirBase, `var`, `run`, defEnvName, `job_http`)
	err = os.MkdirAll(env.dirRunJobHttp, 0700)
	if err != nil {
		return fmt.Errorf(`%s: %w`, env.dirRunJobHttp, err)
	}

	return nil
}

// httpJobsLock lock all the jobs.
func (env *Environment) httpJobsLock() {
	var jobHttp *JobHttp
	for _, jobHttp = range env.HttpJobs {
		jobHttp.Lock()
	}
}

func (env *Environment) httpJobsSave() (err error) {
	var jobHttp *JobHttp
	for _, jobHttp = range env.HttpJobs {
		err = jobHttp.stateSave()
		if err != nil {
			return err
		}
	}
	return nil
}

// httpJobsUnlock unlock all the jobs.
func (env *Environment) httpJobsUnlock() {
	var jobHttp *JobHttp
	for _, jobHttp = range env.HttpJobs {
		jobHttp.Unlock()
	}
}
