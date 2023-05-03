// SPDX-FileCopyrightText: 2021 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

package karajo

import (
	"errors"
	"fmt"
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/shuLhan/share/lib/ascii"
	"github.com/shuLhan/share/lib/ini"
	"github.com/shuLhan/share/lib/mlog"
	libhtml "github.com/shuLhan/share/lib/net/html"
)

const (
	defDirBase       = `/`
	defEnvName       = `karajo`
	defHttpTimeout   = 5 * time.Minute
	defListenAddress = `127.0.0.1:31937`
	defMaxJobRunning = 1
)

// Environment contains configuration for HTTP server, logs, and list of jobs.
type Environment struct {
	// List of Job by name.
	Jobs map[string]*Job `ini:"job" json:"jobs"`

	// jobq is the channel that limit the number of job running at the
	// same time.
	// This limit can be overwritten by MaxJobRunning.
	jobq chan struct{}

	// List of JobHttp by name.
	HttpJobs map[string]*JobHttp `ini:"job.http" json:"http_jobs"`

	// Name of the service.
	// The Name will be used for title on the web user interface, as log
	// prefix, as file prefix on the jobs state, and as file prefix on
	// log files.
	// If this value is empty, it will be set to "karajo".
	Name string `ini:"karajo::name" json:"name"`
	name string

	// Define the address for WUI, default to ":31937".
	ListenAddress string `ini:"karajo::listen_address" json:"listen_address"`

	// DirBase define the base directory where configuration, job state,
	// and job log stored.
	// This field is optional, default to current directory.
	// The structure of directory follow the common UNIX system,
	//
	//	$DirBase
	//	|
	//	+-- /etc/karajo/ +-- karajo.conf
	//	|                +-- job.d/
	//	|                +-- job_http.d/
	//	|
	//	+-- /var/lib/karajo/job/$Job.ID
	//	|
	//	+-- /var/log/karajo/ +-- job/$Job.ID
	//	|                    +-- job_http/$Job.ID
	//	|
	//	+-- /var/run/karajo/job_http/$Job.ID
	//
	// Each job log stored under directory /var/log/karajo/job and the job
	// state under directory /var/run/karajo/job.
	DirBase   string `ini:"karajo::dir_base" json:"dir_base"`
	dirConfig string

	// dirConfigJobd is the directory where job configuration loaded.
	// This is to simplify managing job by splitting it per file.
	// Each job configuration end with `.conf`.
	dirConfigJobd string

	// dirConfigJobHttpd the directory where JobHttp configuration
	// loaded.
	// This is to simplify managing JobHttp by splitting it per file.
	// Each JobHttp configuration end with `.conf`.
	dirConfigJobHttpd string

	dirLibJob string

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
	DirPublic string `ini:"karajo::dir_public" json:"dir_public"`

	// Secret define the default secret to authorize the incoming HTTP
	// request.
	// The signature is generated from HTTP payload (query or body) with
	// HMAC+SHA-256.
	// The signature is read from HTTP header "X-Karajo-Sign" as hex
	// string.
	// This field is optional, if its empty the new secret will be
	// generated and printed to standard output on each run.
	Secret  string `ini:"karajo::secret" json:"-"`
	secretb []byte

	// HttpTimeout define the global HTTP client timeout when executing
	// each jobs.
	// This field is optional, default to 5 minutes.
	// The value of this option is using the Go [time.Duration]
	// format, for example, "30s" for 30 seconds, "1m" for 1 minute.
	HttpTimeout time.Duration `ini:"karajo::http_timeout" json:"http_timeout"`

	// MaxJobRunning define the maximum job running at the same time.
	// This field is optional default to 1.
	MaxJobRunning int `ini:"karajo::max_job_running" json:"max_job_running"`

	// IsDevelopment if its true, the files in DirPublic will be loaded
	// directly from disk instead from embedded memfs.
	IsDevelopment bool `json:"is_development"`
}

// LoadEnvironment load the configuration from the ini file format.
func LoadEnvironment(file string) (env *Environment, err error) {
	var (
		logp = `LoadEnvironment`
		cfg  *ini.Ini
	)

	cfg, err = ini.Open(file)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}

	env = &Environment{
		file: file,
	}

	err = cfg.Unmarshal(env)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}

	return env, nil
}

// NewEnvironment create and initialize new Environment with default values,
// where Name is "karajo", listen address is ":31937", base directory is "/",
// HTTP timeout is 5 minutes, and maximum job running is 1.
func NewEnvironment() (env *Environment) {
	env = &Environment{
		Name:          defEnvName,
		Jobs:          make(map[string]*Job),
		HttpJobs:      make(map[string]*JobHttp),
		ListenAddress: defListenAddress,
		DirBase:       defDirBase,
		HttpTimeout:   defHttpTimeout,
		MaxJobRunning: defMaxJobRunning,
	}
	return env
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

func (env *Environment) init() (err error) {
	var (
		logp = `init`

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
		rand.Seed(time.Now().Unix())
		var secret = ascii.Random([]byte(ascii.LettersNumber), 32)
		env.Secret = string(secret)

		mlog.Outf(`!!! WARNING: Your secret is empty and has been generated: %s`, secret)
	}
	env.secretb = []byte(env.Secret)

	err = env.initDirs()
	if err != nil {
		return fmt.Errorf(`%s: %w`, logp, err)
	}

	err = env.loadJobd()
	if err != nil {
		return fmt.Errorf(`%s: %w`, logp, err)
	}

	for name, job = range env.Jobs {
		err = job.init(env, name)
		if err != nil {
			return fmt.Errorf(`%s: %w`, logp, err)
		}
	}

	err = env.loadJobHttpd()
	if err != nil {
		return fmt.Errorf(`%s: %w`, logp, err)
	}

	for name, jobHttp = range env.HttpJobs {
		err = jobHttp.init(env, name)
		if err != nil {
			return fmt.Errorf(`%s: %w`, logp, err)
		}
	}

	return nil
}

// initDirs create all job and log directories.
func (env *Environment) initDirs() (err error) {
	var (
		logp = `initDirs`
	)

	if len(env.DirBase) == 0 {
		env.DirBase = defDirBase
	}

	env.dirConfig = filepath.Join(env.DirBase, `etc`, defEnvName)
	env.dirConfigJobd = filepath.Join(env.DirBase, `etc`, defEnvName, `job.d`)
	env.dirConfigJobHttpd = filepath.Join(env.DirBase, `etc`, defEnvName, `job_http.d`)

	env.dirLibJob = filepath.Join(env.DirBase, `var`, `lib`, defEnvName, `job`)
	err = os.MkdirAll(env.dirLibJob, 0700)
	if err != nil {
		return fmt.Errorf(`%s: %s: %w`, logp, env.dirLibJob, err)
	}

	env.dirLogJob = filepath.Join(env.DirBase, `var`, `log`, defEnvName, `job`)
	err = os.MkdirAll(env.dirLogJob, 0700)
	if err != nil {
		return fmt.Errorf(`%s: %s: %w`, logp, env.dirLogJob, err)
	}

	env.dirLogJobHttp = filepath.Join(env.DirBase, `var`, `log`, defEnvName, `job_http`)
	err = os.MkdirAll(env.dirLogJobHttp, 0700)
	if err != nil {
		return fmt.Errorf(`%s: %s: %w`, logp, env.dirLogJobHttp, err)
	}

	env.dirRunJobHttp = filepath.Join(env.DirBase, `var`, `run`, defEnvName, `job_http`)
	err = os.MkdirAll(env.dirRunJobHttp, 0700)
	if err != nil {
		return fmt.Errorf(`%s: %s: %w`, logp, env.dirRunJobHttp, err)
	}

	return nil
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

// loadConfigJob load jobs configuration from file.
func (env *Environment) loadConfigJob(conf string) (jobs map[string]*Job, err error) {
	type jobContainer struct {
		Jobs map[string]*Job `ini:"job"`
	}

	var (
		logp = `loadConfigJob`

		cfg *ini.Ini
	)

	cfg, err = ini.Open(conf)
	if err != nil {
		return nil, fmt.Errorf(`%s: %s: %w`, logp, conf, err)
	}

	var jobc = jobContainer{}

	err = cfg.Unmarshal(&jobc)
	if err != nil {
		return nil, fmt.Errorf(`%s: %s: %w`, logp, conf, err)
	}

	jobs = jobc.Jobs
	jobc.Jobs = nil

	return jobs, nil
}

// loadConfigJobHttp load JobHttp configuration from file.
func (env *Environment) loadConfigJobHttp(conf string) (httpJobs map[string]*JobHttp, err error) {
	type jobContainer struct {
		HttpJobs map[string]*JobHttp `ini:"job.http"`
	}

	var (
		logp = `loadConfigJobHttp`

		cfg *ini.Ini
	)

	cfg, err = ini.Open(conf)
	if err != nil {
		return nil, fmt.Errorf(`%s: %s: %w`, logp, conf, err)
	}

	var jobc = jobContainer{}

	err = cfg.Unmarshal(&jobc)
	if err != nil {
		return nil, fmt.Errorf(`%s: %s: %w`, logp, conf, err)
	}

	httpJobs = jobc.HttpJobs
	jobc.HttpJobs = nil

	return httpJobs, nil
}

// loadJobd load all job configurations from a directory.
func (env *Environment) loadJobd() (err error) {
	var (
		logp = `loadJobd`

		jobd    *os.File
		listde  []os.DirEntry
		de      os.DirEntry
		fm      os.FileMode
		name    string
		jobConf string
		jobs    map[string]*Job
		job     *Job
	)

	jobd, err = os.Open(env.dirConfigJobd)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf(`%s: %w`, logp, err)
	}

	listde, err = jobd.ReadDir(0)
	if err != nil {
		return fmt.Errorf(`%s: %w`, logp, err)
	}

	if env.Jobs == nil {
		env.Jobs = make(map[string]*Job)
	}

	for _, de = range listde {
		if de.IsDir() {
			continue
		}
		fm = de.Type()
		if !fm.IsRegular() {
			continue
		}
		name = de.Name()

		if name[0] == '.' {
			// Exclude hidden file.
			continue
		}
		if !strings.HasSuffix(name, `.conf`) {
			continue
		}

		jobConf = filepath.Join(env.dirConfigJobd, name)

		jobs, err = env.loadConfigJob(jobConf)
		if err != nil {
			return fmt.Errorf(`%s: %w`, logp, err)
		}

		for name, job = range jobs {
			env.Jobs[name] = job
		}
	}
	return nil
}

// loadJobHttpd load all JobHttp configurations from a directory.
func (env *Environment) loadJobHttpd() (err error) {
	var (
		logp = `loadJobHttpd`

		jobd     *os.File
		listde   []os.DirEntry
		de       os.DirEntry
		fm       os.FileMode
		name     string
		fileConf string
		httpJobs map[string]*JobHttp
		httpJob  *JobHttp
	)

	jobd, err = os.Open(env.dirConfigJobHttpd)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf(`%s: %w`, logp, err)
	}

	listde, err = jobd.ReadDir(0)
	if err != nil {
		return fmt.Errorf(`%s: %w`, logp, err)
	}

	if env.HttpJobs == nil {
		env.HttpJobs = make(map[string]*JobHttp)
	}

	for _, de = range listde {
		if de.IsDir() {
			continue
		}
		fm = de.Type()
		if !fm.IsRegular() {
			continue
		}
		name = de.Name()

		if name[0] == '.' {
			// Exclude hidden file.
			continue
		}
		if !strings.HasSuffix(name, `.conf`) {
			continue
		}

		fileConf = filepath.Join(env.dirConfigJobHttpd, name)

		httpJobs, err = env.loadConfigJobHttp(fileConf)
		if err != nil {
			return fmt.Errorf(`%s: %w`, logp, err)
		}

		for name, httpJob = range httpJobs {
			env.HttpJobs[name] = httpJob
		}
	}
	return nil
}

func (env *Environment) lockAllJob() {
	var job *Job
	for _, job = range env.Jobs {
		job.Lock()
	}

	var jobHttp *JobHttp
	for _, jobHttp = range env.HttpJobs {
		jobHttp.Lock()
	}
}
func (env *Environment) unlockAllJob() {
	var job *Job
	for _, job = range env.Jobs {
		job.Unlock()
	}

	var jobHttp *JobHttp
	for _, jobHttp = range env.HttpJobs {
		jobHttp.Unlock()
	}
}
