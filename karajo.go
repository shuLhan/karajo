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
	"fmt"
	"hash"
	"net/http"
	"time"

	liberrors "github.com/shuLhan/share/lib/errors"
	libhttp "github.com/shuLhan/share/lib/http"
	"github.com/shuLhan/share/lib/memfs"
	"github.com/shuLhan/share/lib/mlog"
)

// Version of this library and program.
const Version = `0.7.1-dev`

// TimeNow return the current time.
// It can be used in testing to provide static, predictable time.
var TimeNow = func() time.Time {
	return time.Now()
}

var (
	memfsWww *memfs.MemFS

	errUnauthorized = liberrors.E{
		Code:    http.StatusUnauthorized,
		Message: `empty or invalid signature`,
	}
)

// Karajo HTTP server and jobs manager.
type Karajo struct {
	httpd *libhttp.Server
	env   *Environment
	sm    *sessionManager
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
	var logp = `New`

	err = env.init()
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}

	k = &Karajo{
		env: env,
		sm:  newSessionManager(),
	}

	mlog.SetPrefix(env.Name + `:`)

	err = k.initMemfs()
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}

	err = k.initHttpd()
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}

	return k, nil
}

// initMemfs initialize the memory file system for serving the WUI and public
// directory.
func (k *Karajo) initMemfs() (err error) {
	var logp = `initMemfs`

	if memfsWww == nil {
		return fmt.Errorf(`%s: empty embedded www`, logp)
	}

	memfsWww.Opts.TryDirect = k.env.IsDevelopment

	if len(k.env.DirPublic) == 0 {
		return nil
	}

	var (
		opts = memfs.Options{
			Root:      k.env.DirPublic,
			TryDirect: true,
		}
		memfsPublic *memfs.MemFS
	)

	memfsPublic, err = memfs.New(&opts)
	if err != nil {
		return fmt.Errorf(`%s: %w`, logp, err)
	}

	memfsWww = memfs.Merge(memfsWww, memfsPublic)
	memfsWww.Root.SysPath = k.env.DirPublic
	memfsWww.Opts.TryDirect = true

	return nil
}

// Start all the jobs and the HTTP server.
func (k *Karajo) Start() (err error) {
	var (
		jobHttp *JobHttp
		job     *Job
	)

	mlog.Outf(`started the karajo server at http://%s/karajo`, k.httpd.Addr)

	for _, job = range k.env.Jobs {
		go job.Start()
	}
	for _, jobHttp = range k.env.HttpJobs {
		go jobHttp.Start()
	}

	return k.httpd.Start()
}

// Stop all the jobs and the HTTP server.
func (k *Karajo) Stop() (err error) {
	var (
		jobHttp *JobHttp
		job     *Job
	)

	for _, jobHttp = range k.env.HttpJobs {
		jobHttp.Stop()
	}
	for _, job = range k.env.Jobs {
		job.Stop()
	}

	return k.httpd.Stop(5 * time.Second)
}
