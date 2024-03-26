// SPDX-FileCopyrightText: 2021 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

package karajo

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"git.sr.ht/~shulhan/pakakeh.go/lib/ascii"
	libhtml "git.sr.ht/~shulhan/pakakeh.go/lib/html"
	"git.sr.ht/~shulhan/pakakeh.go/lib/ini"
	"git.sr.ht/~shulhan/pakakeh.go/lib/mlog"
)

const (
	defDirBase       = `/`
	defEnvName       = `karajo`
	defHTTPTimeout   = 5 * time.Minute
	defListenAddress = `127.0.0.1:31937`
	defMaxJobRunning = 1
)

// Env contains configuration for HTTP server, logs, and list of jobs.
type Env struct {
	// List of JobExec by name.
	ExecJobs map[string]*JobExec `ini:"job" json:"jobs"`

	// List of JobHTTP by name.
	HTTPJobs map[string]*JobHTTP `ini:"job.http" json:"http_jobs"`

	// Notif contains list of notification setting.
	Notif map[string]EnvNotif `ini:"notif" json:"-"`

	// Index of notification client by its name.
	notif map[string]notifClient

	// Users list of user that can access web user interface.
	// The list of user optionally loaded from
	// $DirBase/etc/karajo/user.conf if the file exist.
	Users map[string]*User `json:"-"`

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
	//	+-- /var/lib/karajo/ +-- job/$JobExec.ID
	//	|                    +-- job_http/$JobHTTP.ID
	//	|
	//	+-- /var/log/karajo/ +-- job/$JobExec.ID
	//	|                    +-- job_http/$JobHTTP.ID
	//	|
	//	+-- /var/run/karajo/job_http/$JobHTTP.ID
	//
	// Each job log stored under directory /var/log/karajo/job and the job
	// state under directory /var/run/karajo/job.
	DirBase string `ini:"karajo::dir_base" json:"dir_base"`

	// Equal to $DirBase/etc/karajo/
	dirConfig string

	// dirConfigJobd is the directory where job configuration loaded.
	// This is to simplify managing job by splitting it per file.
	// Each job configuration end with `.conf`.
	dirConfigJobd string

	// dirConfigJobHTTPd the directory where JobHTTP configuration
	// loaded.
	// This is to simplify managing JobHTTP by splitting it per file.
	// Each JobHTTP configuration end with `.conf`.
	dirConfigJobHTTPd string

	dirLibJob     string
	dirLibJobHTTP string

	dirLogJob     string
	dirLogJobHTTP string

	// dirRunJobHTTP define the directory where JobHTTP state is stored.
	dirRunJobHTTP string

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

	// HTTPTimeout define the global HTTP client timeout when executing
	// each jobs.
	// This field is optional, default to 5 minutes.
	// The value of this option is using the Go [time.Duration]
	// format, for example, "30s" for 30 seconds, "1m" for 1 minute.
	HTTPTimeout time.Duration `ini:"karajo::http_timeout" json:"http_timeout"`

	// MaxJobRunning define the maximum job running at the same time.
	// This field is optional default to 1.
	MaxJobRunning int `ini:"karajo::max_job_running" json:"max_job_running"`

	// IsDevelopment if its true, the files in DirPublic will be loaded
	// directly from disk instead from embedded memfs.
	IsDevelopment bool `json:"is_development"`
}

// LoadEnv load the configuration from the ini file format.
func LoadEnv(file string) (env *Env, err error) {
	var (
		logp = `LoadEnv`
		cfg  *ini.Ini
	)

	cfg, err = ini.Open(file)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}

	env = &Env{
		file: file,
	}

	err = cfg.Unmarshal(env)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}

	return env, nil
}

// NewEnv create and initialize new Env with default values,
// where Name is "karajo", listen address is ":31937", base directory is "/",
// HTTP timeout is 5 minutes, and maximum job running is 1.
func NewEnv() (env *Env) {
	env = &Env{
		Name:          defEnvName,
		ExecJobs:      make(map[string]*JobExec),
		HTTPJobs:      make(map[string]*JobHTTP),
		Users:         make(map[string]*User),
		ListenAddress: defListenAddress,
		DirBase:       defDirBase,
		HTTPTimeout:   defHTTPTimeout,
		MaxJobRunning: defMaxJobRunning,
	}
	return env
}

// ParseEnv parse the environment from raw bytes.
func ParseEnv(content []byte) (env *Env, err error) {
	var (
		logp = `ParseEnv`
	)

	env = &Env{}

	err = ini.Unmarshal(content, env)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}

	return env, nil
}

// jobExec get the JobExec by its ID.
func (env *Env) jobExec(id string) (job *JobExec) {
	for _, job = range env.ExecJobs {
		if job.ID == id {
			return job
		}
	}
	return nil
}

// jobHTTP get the registered JobHTTP by its ID.
func (env *Env) jobHTTP(id string) (job *JobHTTP) {
	for _, job = range env.HTTPJobs {
		if job.ID == id {
			return job
		}
	}
	return nil
}

func (env *Env) init() (err error) {
	var (
		logp = `init`

		job     *JobExec
		jobHTTP *JobHTTP
		name    string
	)

	if len(env.Name) == 0 {
		env.Name = defEnvName
	}
	env.name = libhtml.NormalizeForID(env.Name)

	if len(env.ListenAddress) == 0 {
		env.ListenAddress = defListenAddress
	}
	if env.HTTPTimeout == 0 {
		env.HTTPTimeout = defHTTPTimeout
	}
	if env.MaxJobRunning <= 0 {
		env.MaxJobRunning = defMaxJobRunning
	}

	if len(env.Secret) == 0 {
		var secret = ascii.Random([]byte(ascii.LettersNumber), 32)
		env.Secret = string(secret)

		mlog.Outf(`!!! WARNING: Your secret is empty and has been generated: %s`, secret)
	}
	env.secretb = []byte(env.Secret)

	err = env.initDirs()
	if err != nil {
		return fmt.Errorf(`%s: %w`, logp, err)
	}

	err = env.initNotifs()
	if err != nil {
		return fmt.Errorf(`%s: %w`, logp, err)
	}

	err = env.initUsers()
	if err != nil {
		return fmt.Errorf(`%s: %w`, logp, err)
	}

	err = env.loadJobd()
	if err != nil {
		return fmt.Errorf(`%s: %w`, logp, err)
	}

	for name, job = range env.ExecJobs {
		err = job.init(env, name)
		if err != nil {
			return fmt.Errorf(`%s: %w`, logp, err)
		}
	}

	err = env.loadJobHTTPd()
	if err != nil {
		return fmt.Errorf(`%s: %w`, logp, err)
	}

	for name, jobHTTP = range env.HTTPJobs {
		err = jobHTTP.init(env, name)
		if err != nil {
			return fmt.Errorf(`%s: %w`, logp, err)
		}
	}

	return nil
}

// initDirs create all job and log directories.
func (env *Env) initDirs() (err error) {
	var (
		logp = `initDirs`
	)

	if len(env.DirBase) == 0 {
		env.DirBase = defDirBase
	}

	env.dirConfig = filepath.Join(env.DirBase, `etc`, defEnvName)
	env.dirConfigJobd = filepath.Join(env.DirBase, `etc`, defEnvName, `job.d`)
	env.dirConfigJobHTTPd = filepath.Join(env.DirBase, `etc`, defEnvName, `job_http.d`)

	env.dirLibJob = filepath.Join(env.DirBase, `var`, `lib`, defEnvName, `job`)
	err = os.MkdirAll(env.dirLibJob, 0700)
	if err != nil {
		return fmt.Errorf(`%s: %s: %w`, logp, env.dirLibJob, err)
	}

	env.dirLibJobHTTP = filepath.Join(env.DirBase, `var`, `lib`, defEnvName, `job_http`)
	err = os.MkdirAll(env.dirLibJobHTTP, 0700)
	if err != nil {
		return fmt.Errorf(`%s: %s: %w`, logp, env.dirLibJobHTTP, err)
	}

	env.dirLogJob = filepath.Join(env.DirBase, `var`, `log`, defEnvName, `job`)
	err = os.MkdirAll(env.dirLogJob, 0700)
	if err != nil {
		return fmt.Errorf(`%s: %s: %w`, logp, env.dirLogJob, err)
	}

	env.dirLogJobHTTP = filepath.Join(env.DirBase, `var`, `log`, defEnvName, `job_http`)
	err = os.MkdirAll(env.dirLogJobHTTP, 0700)
	if err != nil {
		return fmt.Errorf(`%s: %s: %w`, logp, env.dirLogJobHTTP, err)
	}

	env.dirRunJobHTTP = filepath.Join(env.DirBase, `var`, `run`, defEnvName, `job_http`)
	err = os.MkdirAll(env.dirRunJobHTTP, 0700)
	if err != nil {
		return fmt.Errorf(`%s: %s: %w`, logp, env.dirRunJobHTTP, err)
	}

	return nil
}

// initNotifs initialize the notification.
func (env *Env) initNotifs() (err error) {
	var (
		logp = `initNotifs`

		name        string
		envNotif    EnvNotif
		clientNotif notifClient
	)
	env.notif = make(map[string]notifClient)
	for name, envNotif = range env.Notif {
		envNotif.Name = name

		envNotif.init()

		clientNotif, err = envNotif.createClient()
		if err != nil {
			return fmt.Errorf(`%s: %w`, logp, err)
		}
		env.notif[name] = clientNotif
	}
	return nil
}

// initUsers load users for authentication from $DirBase/etc/karajo/user.conf.
func (env *Env) initUsers() (err error) {
	var (
		logp         = `initUsers`
		fileUserConf = filepath.Join(env.dirConfig, `user.conf`)

		listUser map[string]*User
		name     string
		u        *User
	)

	listUser, err = loadUsers(fileUserConf)
	if err != nil {
		return fmt.Errorf(`%s: %w`, logp, err)
	}

	mlog.Outf(`Loaded %d users from %s.`, len(listUser), fileUserConf)

	if env.Users == nil {
		env.Users = make(map[string]*User)
	}
	for name, u = range listUser {
		env.Users[name] = u
	}

	return nil
}

// loadConfigJob load jobs configuration from file.
//
// The conf file can contains one or more jobs configuration.
func (env *Env) loadConfigJob(conf string) (jobs map[string]*JobExec, err error) {
	type jobContainer struct {
		ExecJobs map[string]*JobExec `ini:"job"`
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

	jobs = jobc.ExecJobs
	jobc.ExecJobs = nil

	return jobs, nil
}

// loadConfigJobHTTP load JobHTTP configuration from file.
func (env *Env) loadConfigJobHTTP(conf string) (httpJobs map[string]*JobHTTP, err error) {
	type jobContainer struct {
		HTTPJobs map[string]*JobHTTP `ini:"job.http"`
	}

	var (
		logp = `loadConfigJobHTTP`

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

	httpJobs = jobc.HTTPJobs
	jobc.HTTPJobs = nil

	return httpJobs, nil
}

// loadJobd load all job configurations from a directory.
func (env *Env) loadJobd() (err error) {
	var (
		logp = `loadJobd`

		jobd    *os.File
		listde  []os.DirEntry
		de      os.DirEntry
		fm      os.FileMode
		name    string
		jobConf string
		jobs    map[string]*JobExec
		job     *JobExec
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

	if env.ExecJobs == nil {
		env.ExecJobs = make(map[string]*JobExec)
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
			env.ExecJobs[name] = job
		}
	}
	return nil
}

// loadJobHTTPd load all JobHTTP configurations from a directory.
func (env *Env) loadJobHTTPd() (err error) {
	var (
		logp = `loadJobHTTPd`

		jobd     *os.File
		listde   []os.DirEntry
		de       os.DirEntry
		fm       os.FileMode
		name     string
		fileConf string
		httpJobs map[string]*JobHTTP
		httpJob  *JobHTTP
	)

	jobd, err = os.Open(env.dirConfigJobHTTPd)
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

	if env.HTTPJobs == nil {
		env.HTTPJobs = make(map[string]*JobHTTP)
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

		fileConf = filepath.Join(env.dirConfigJobHTTPd, name)

		httpJobs, err = env.loadConfigJobHTTP(fileConf)
		if err != nil {
			return fmt.Errorf(`%s: %w`, logp, err)
		}

		for name, httpJob = range httpJobs {
			env.HTTPJobs[name] = httpJob
		}
	}
	return nil
}

func (env *Env) lockAllJob() {
	var job *JobExec
	for _, job = range env.ExecJobs {
		job.Lock()
	}

	var jobHTTP *JobHTTP
	for _, jobHTTP = range env.HTTPJobs {
		jobHTTP.Lock()
	}
}

func (env *Env) unlockAllJob() {
	var job *JobExec
	for _, job = range env.ExecJobs {
		job.Unlock()
	}

	var jobHTTP *JobHTTP
	for _, jobHTTP = range env.HTTPJobs {
		jobHTTP.Unlock()
	}
}
