// SPDX-FileCopyrightText: 2021 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

// Package karajo implement HTTP workers and manager similar to cron but
// works only on HTTP.
//
// karajo has the web user interface (WUI) for monitoring the jobs that run
// on port 31937 by default and can be configurable.
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
	"net/http"
	"time"

	liberrors "github.com/shuLhan/share/lib/errors"
	libhttp "github.com/shuLhan/share/lib/http"
	"github.com/shuLhan/share/lib/memfs"
	"github.com/shuLhan/share/lib/mlog"
)

// Version of this library and program.
const Version = `0.9.0`

// timeNow return the current time in UTC rounded to second.
// During testing the variable will be replaced to provide static,
// predictable time.
var timeNow = func() time.Time {
	return time.Now().Round(time.Second).UTC()
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
	// HTTPd the HTTP server that Karajo use.
	// One can register additional endpoints here.
	HTTPd *libhttp.Server

	env *Env
	sm  *sessionManager

	// jobq is the channel that limit the number of job running at the
	// same time.
	// This limit can be overwritten by MaxJobRunning.
	jobq chan struct{}

	// logq is used to collect all job log once they finished.
	logq chan *JobLog
}

// Sign generate hex string of HMAC + SHA256 of payload using the secret.
func Sign(payload, secret []byte) (sign string) {
	var signer = hmac.New(sha256.New, secret)

	_, _ = signer.Write(payload)
	var bsign = signer.Sum(nil)
	sign = hex.EncodeToString(bsign)
	return sign
}

// New create and initialize Karajo from configuration file.
func New(env *Env) (k *Karajo, err error) {
	var logp = `New`

	err = env.init()
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}

	k = &Karajo{
		env:  env,
		sm:   newSessionManager(),
		jobq: make(chan struct{}, env.MaxJobRunning),
		logq: make(chan *JobLog),
	}

	mlog.SetPrefix(env.Name + `:`)

	err = k.initMemfs()
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}

	err = k.initHTTPd()
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
		jobHTTP *JobHTTP
		job     *JobExec
	)

	mlog.Outf(`started the karajo server at http://%s/karajo`, k.HTTPd.Addr)

	if len(k.env.notif) > 0 {
		go k.workerNotification()
	}

	for _, job = range k.env.ExecJobs {
		go job.Start(k.jobq, k.logq)
		<-k.jobq
	}
	for _, jobHTTP = range k.env.HTTPJobs {
		go jobHTTP.Start(k.jobq, k.logq)
		<-k.jobq
	}

	return k.HTTPd.Start()
}

// Stop all the jobs and the HTTP server.
func (k *Karajo) Stop() (err error) {
	var (
		jobHTTP *JobHTTP
		job     *JobExec
	)

	for _, jobHTTP = range k.env.HTTPJobs {
		jobHTTP.Stop()
	}
	for _, job = range k.env.ExecJobs {
		job.Stop()
	}

	return k.HTTPd.Stop(5 * time.Second)
}

// workerNotification receive JobLog from JobExec and JobHTTP everytime
// their started, running, success, failed, or paused.
func (k *Karajo) workerNotification() {
	var (
		jlog         *JobLog
		clientNotif  notifClient
		notifName    string
		logNotifName string
	)
	for jlog = range k.logq {
		for _, logNotifName = range jlog.listNotif {
			for notifName, clientNotif = range k.env.notif {
				if logNotifName != notifName {
					continue
				}
				go clientNotif.Send(jlog)
			}
		}
	}
}
