// SPDX-FileCopyrightText: 2022 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

package karajo

import (
	"bytes"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	libhttp "github.com/shuLhan/share/lib/http"
	"github.com/shuLhan/share/lib/mlog"
	libhtml "github.com/shuLhan/share/lib/net/html"
)

const (
	defJobLogRetention = 5

	jobEnvCounter   = `KARAJO_JOB_COUNTER`
	jobEnvPath      = `PATH`
	jobEnvPathValue = `/usr/local/sbin:/usr/local/bin:/usr/bin:/usr/bin/site_perl:/usr/bin/vendor_perl:/usr/bin/core_perl`
)

// List of Job AuthKind for authorization.
const (
	JobAuthKindGithub     = `github`
	JobAuthKindHmacSha256 = `hmac-sha256` // Default AuthKind if not set.
	JobAuthKindSourcehut  = `sourcehut`
)

const (
	githubHeaderSign256 = `X-Hub-Signature-256`
	githubHeaderSign    = `X-Hub-Signature`

	sourcehutHeaderSign  = `X-Payload-Signature`
	sourcehutHeaderNonce = `X-Payload-Nonce`
	sourcehutPublicKey   = `uX7KWyyDNMaBma4aVbJ/cbUQpdjqczuCyK/HxzV/u+4=`
)

// JobHttpHandler define an handler for triggering a Job using HTTP.
//
// The log parameter is used to log all output and error.
// The epr parameter contains HTTP request, body, and response writer.
type JobHttpHandler func(log io.Writer, epr *libhttp.EndpointRequest) error

// Job a job can be triggered manually by sending HTTP POST request or
// automatically by timer (per interval).
//
// For job triggered by HTTP request, the Path and Secret must be set.
// For job triggered by timer, the Interval must be positive duration, equal
// or greater than 1 minute.
//
// Each Job contains a working directory, and a callback or list of commands
// to be executed.
type Job struct {
	// Shared Environment.
	env *Environment `json:"-"`

	// Cache of log sorted by its counter.
	Logs []*JobLog

	// Call define a function or method to be called, as an
	// alternative to Commands.
	// This field is optional, it is only used if Job created through
	// code.
	Call JobHttpHandler `json:"-" ini:"-"`

	// HTTP path where Job can be triggered using HTTP.
	// The Path is automatically prefixed with "/karajo/api/job/run", it
	// is not static.
	// For example, if it set to "/my", then the actual path would be
	// "/karajo/api/job/run/my".
	// This field is optional and unique between Job.
	Path string `ini:"::path"`

	// Supported AuthKind are,
	//
	//   - github: the signature read from "x-hub-signature-256" and
	//     compare it by signing request body with Secret using
	//     HMAC-SHA256.
	//     If the header is empty, it will check another header
	//     "x-hub-signature" and then sign the request body with Secret
	//     using HMAC-SHA1.
	//
	//   - hmac-sha256: the signature read from HeaderSign and compare it
	//     by signing request body with Secret using HMAC-SHA256.
	//
	//   - sourcehut: See https://man.sr.ht/api-conventions.md#webhooks
	//
	// If this field is empty or invalid it will be set to hmac-sha256.
	AuthKind string `ini:"::auth_kind"`

	// HeaderSign define the HTTP header where the signature is read.
	// Default to "X-Karajo-Sign" if its empty.
	HeaderSign string `ini:"::header_sign"`

	// Secret define a string to validate the signature of request.
	// If its empty, it will be set to global Secret from Environment.
	Secret string `ini:"::secret" json:"-"`

	// dirWork define the directory on the system where all commands
	// will be executed.
	dirWork string
	dirLog  string

	// Commands list of command to be executed.
	// This option can be defined multiple times.
	// The following environment variables are available inside the
	// command:
	//
	//   - KARAJO_JOB_COUNTER: contains the current job counter.
	Commands []string `ini:"::command"`

	JobBase

	// LogRetention define the maximum number of logs to keep in storage.
	// This field is optional, default to 5.
	LogRetention int `ini:"::log_retention"`

	lastCounter int64
}

// authorize the hook based on the AuthKind.
func (job *Job) authorize(headers http.Header, reqbody []byte) (err error) {
	var (
		logp = `authorize`
	)

	switch job.AuthKind {
	case JobAuthKindGithub:
		err = job.authGithub(headers, reqbody)

	case JobAuthKindSourcehut:
		var (
			pub ed25519.PublicKey
		)
		pub, err = decodeSourcehutPublicKey()
		if err != nil {
			return fmt.Errorf(`%s: %w`, logp, err)
		}
		err = job.authSourcehut(headers, reqbody, pub)

	default:
		err = job.authHmacSha256(headers, reqbody)
	}
	if err != nil {
		return fmt.Errorf(`%s: %w`, logp, err)
	}
	return nil
}

// authGithub authorize the Github Webhook request.
func (job *Job) authGithub(headers http.Header, reqbody []byte) (err error) {
	var (
		logp    = `authGithub`
		gotSign = headers.Get(githubHeaderSign256)
		secret  = []byte(job.Secret)

		expSign string
	)

	if len(gotSign) != 0 {
		gotSign = strings.TrimPrefix(gotSign, `sha256=`)
		expSign = Sign(reqbody, secret)
	} else {
		gotSign = headers.Get(githubHeaderSign)
		expSign = signHmacSha1(reqbody, secret)
	}
	if expSign != gotSign {
		return fmt.Errorf(`%s: %w`, logp, ErrJobForbidden)
	}

	return nil
}

// authGithub authorize the Sourcehut Webhook request.
func (job *Job) authSourcehut(headers http.Header, reqbody []byte, pubkey ed25519.PublicKey) (err error) {
	var (
		logp    = `authSourcehut`
		signb64 = headers.Get(sourcehutHeaderSign)
	)

	if len(signb64) == 0 {
		return fmt.Errorf(`%s: empty header sign: %w`, logp, ErrJobForbidden)
	}

	var sign []byte

	sign, err = base64.StdEncoding.DecodeString(signb64)
	if err != nil {
		return fmt.Errorf(`%s: invalid header sign: %w`, logp, err)
	}

	var (
		nonce = headers.Get(sourcehutHeaderNonce)

		msg bytes.Buffer
	)

	msg.Write(reqbody)
	msg.WriteString(nonce)

	if !ed25519.Verify(pubkey, msg.Bytes(), sign) {
		return fmt.Errorf(`%s: %w`, logp, ErrJobForbidden)
	}

	return nil
}

// authGithub authorize custom Webhook using signature from HeaderSign.
//
// The signature is generated using HMAC-SHA256 algorithm using Secret as key
// and request body as message.
func (job *Job) authHmacSha256(headers http.Header, reqbody []byte) (err error) {
	var (
		logp    = `authHmacSha256`
		gotSign = headers.Get(job.HeaderSign)
	)
	if len(gotSign) == 0 {
		return fmt.Errorf(`%s: empty header sign: %s: %w`, logp,
			job.HeaderSign, ErrJobForbidden)
	}

	var (
		secret  = []byte(job.Secret)
		expSign = Sign(reqbody, secret)
	)
	if gotSign != expSign {
		return fmt.Errorf(`%s: %w`, logp, ErrJobForbidden)
	}

	return nil
}

func (job *Job) generateCmdEnvs() (env []string) {
	env = append(env, fmt.Sprintf(`%s=%d`, jobEnvCounter, job.lastCounter))
	env = append(env, fmt.Sprintf(`%s=%s`, jobEnvPath, jobEnvPathValue))
	return env
}

// init initialize the Job.
//
// For Job that need to be triggered by HTTP request the Path and Secret
// _must_ not be empty.
// If Secret is not set then it will default to Environment's Secret.
//
// It will return an error ErrJobEmptyCommandsOrCall if one of the Call or
// Commands is not set.
func (job *Job) init(env *Environment, name string) (err error) {
	job.JobBase.init()

	job.Path = strings.TrimSpace(job.Path)
	job.Secret = strings.TrimSpace(job.Secret)
	if len(job.Secret) == 0 {
		job.Secret = env.Secret
	}

	if len(job.Commands) == 0 && job.Call == nil {
		return ErrJobEmptyCommandsOrCall
	}

	job.env = env
	job.Name = name
	job.ID = libhtml.NormalizeForID(name)
	if job.LogRetention <= 0 {
		job.LogRetention = defJobLogRetention
	}

	err = job.initDirsState(env)
	if err != nil {
		return err
	}

	err = job.initLogs()
	if err != nil {
		return err
	}

	job.initTimer()

	if len(job.HeaderSign) == 0 {
		job.HeaderSign = HeaderNameXKarajoSign
	}

	job.AuthKind = strings.ToLower(job.AuthKind)

	switch job.AuthKind {
	case JobAuthKindGithub, JobAuthKindSourcehut, JobAuthKindHmacSha256:
		// OK.
	default:
		job.AuthKind = JobAuthKindHmacSha256
	}

	return nil
}

func (job *Job) initDirsState(env *Environment) (err error) {
	job.dirWork = filepath.Join(env.dirLibJob, job.ID)
	err = os.MkdirAll(job.dirWork, 0700)
	if err != nil {
		return err
	}

	job.dirLog = filepath.Join(env.dirLogJob, job.ID)
	err = os.MkdirAll(job.dirLog, 0700)
	if err != nil {
		return err
	}

	return nil
}

// initLogs load the job logs state, counter and status.
func (job *Job) initLogs() (err error) {
	var (
		dir       *os.File
		hlog      *JobLog
		fi        os.FileInfo
		fiModTime time.Time
		fis       []os.FileInfo
	)

	dir, err = os.Open(job.dirLog)
	if err != nil {
		return err
	}
	fis, err = dir.Readdir(0)
	if err != nil {
		return err
	}

	for _, fi = range fis {
		hlog = parseJobLogName(job.dirLog, fi.Name())
		if hlog == nil {
			// Skip log with invalid file name.
			continue
		}

		job.Logs = append(job.Logs, hlog)

		if hlog.Counter > job.lastCounter {
			job.lastCounter = hlog.Counter
			job.Status = hlog.Status
		}

		fiModTime = fi.ModTime()
		if job.LastRun.IsZero() {
			job.LastRun = fiModTime
		} else if fiModTime.After(job.LastRun) {
			job.LastRun = fiModTime
		}
	}

	job.LastRun = job.LastRun.UTC().Round(time.Second)

	sort.Slice(job.Logs, func(x, y int) bool {
		return job.Logs[x].Counter < job.Logs[y].Counter
	})

	job.logsPrune()

	return nil
}

// initTimer init fields that required to run Job with interval.
func (job *Job) initTimer() {
	if job.Interval <= 0 {
		return
	}
	if job.Interval < time.Minute {
		job.Interval = time.Minute
	}
}

func (job *Job) logsPrune() {
	var (
		hlog     *JobLog
		totalLog int
		indexMin int
	)

	totalLog = len(job.Logs)
	if totalLog > job.LogRetention {
		// Delete old logs.
		indexMin = totalLog - job.LogRetention
		for _, hlog = range job.Logs[:indexMin] {
			_ = os.Remove(hlog.path)
		}
		job.Logs = job.Logs[indexMin:]
	}
}

// handleHttp handle trigger to run the Job from HTTP request.
//
// Once the signature is verified it will response immediately and run the
// actual process in the new goroutine.
func (job *Job) handleHttp(epr *libhttp.EndpointRequest) (resbody []byte, err error) {
	var (
		logp = `handleHttp`
	)

	// Authenticated request by checking the request body.
	err = job.authorize(epr.HttpRequest.Header, epr.RequestBody)
	if err != nil {
		return nil, fmt.Errorf(`%s: %s: %w`, logp, job.ID, err)
	}

	err = job.start()
	if err != nil {
		return nil, fmt.Errorf(`%s: %s: %w`, logp, job.ID, err)
	}

	go func() {
		var (
			jlog *JobLog
			err  error
		)
		jlog, err = job.execute(epr)
		if err != nil {
			mlog.Errf(`!!! job: %s: failed: %s.`, job.ID, err)
		} else {
			mlog.Outf(`job: %s: finished.`, job.ID)
		}
		job.finish(jlog, err)
	}()

	var res libhttp.EndpointResponse

	res.Code = http.StatusOK
	res.Message = `OK`
	res.Data = job

	job.Lock()
	resbody, err = json.Marshal(&res)
	job.Unlock()

	return resbody, err
}

// Start the Job timer only if its Interval is non-zero.
func (job *Job) Start() {
	if job.Interval <= 0 {
		return
	}

	var (
		now          time.Time
		nextInterval time.Duration
		timer        *time.Timer
		jlog         *JobLog
		err          error
		ever         bool
	)

	for {
		job.Lock()
		now = TimeNow().UTC().Round(time.Second)
		nextInterval = job.computeNextInterval(now)
		job.NextRun = now.Add(nextInterval)
		job.Unlock()

		mlog.Outf(`job: %s: next running in %s ...`, job.ID, nextInterval)

		timer = time.NewTimer(nextInterval)
		ever = true
		for ever {
			select {
			case <-timer.C:
				err = job.start()
				if err != nil {
					mlog.Errf(`!!! job: %s: %s`, job.ID, err)
					timer.Stop()
					ever = false
					continue
				}

				jlog, err = job.execute(nil)
				if err != nil {
					mlog.Errf(`!!! job: %s: failed: %s.`, job.ID, err)
				} else {
					mlog.Outf(`job: %s: finished.`, job.ID)
				}
				job.finish(jlog, err)
				// The finish will trigger the finished channel.

			case <-job.finished:
				timer.Stop()
				ever = false

			case <-job.stopped:
				timer.Stop()
				return
			}
		}
	}
}

// execute the job Call or commands.
func (job *Job) execute(epr *libhttp.EndpointRequest) (jlog *JobLog, err error) {
	var (
		now = TimeNow().UTC().Round(time.Second)

		execCmd exec.Cmd
		logTime string
		cmd     string
		x       int
	)

	job.env.jobq <- struct{}{}
	mlog.Outf(`job: %s: started ...`, job.ID)
	defer func() {
		<-job.env.jobq
	}()

	job.Lock()
	job.Status = JobStatusRunning
	job.lastCounter++
	jlog = newJobLog(job.ID, job.dirLog, job.lastCounter)
	job.Logs = append(job.Logs, jlog)
	job.logsPrune()
	job.Unlock()

	mlog.Outf(`job: %s: running ...`, job.ID)

	// Call the job.
	if job.Call != nil {
		err = job.Call(jlog, epr)
		return jlog, err
	}

	// Run commands.
	for x, cmd = range job.Commands {
		now = TimeNow().UTC().Round(time.Second)
		logTime = now.Format(defTimeLayout)
		fmt.Fprintf(jlog, "\n%s === Execute %2d: %s\n", logTime, x, cmd)

		execCmd = exec.Cmd{
			Path:   `/bin/sh`,
			Dir:    job.dirWork,
			Args:   []string{`/bin/sh`, `-c`, cmd},
			Env:    job.generateCmdEnvs(),
			Stdout: jlog,
			Stderr: jlog,
		}

		err = execCmd.Run()
		if err != nil {
			return jlog, err
		}
	}

	return jlog, nil
}

// Stop the Job timer execution.
func (job *Job) Stop() {
	mlog.Outf(`job: %s: stopping ...`, job.ID)

	select {
	case job.stopped <- true:
	default:
	}
}

func decodeSourcehutPublicKey() (pubkey ed25519.PublicKey, err error) {
	var (
		logp = `decodeSourcehutPublicKey`

		pubkeyb []byte
	)

	pubkeyb, err = base64.StdEncoding.DecodeString(sourcehutPublicKey)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}

	pubkey = ed25519.PublicKey(pubkeyb)

	return pubkey, nil
}

func signHmacSha1(payload, secret []byte) (sign string) {
	var (
		signer hash.Hash = hmac.New(sha1.New, secret)
		bsign  []byte
	)
	_, _ = signer.Write(payload)
	bsign = signer.Sum(nil)
	sign = hex.EncodeToString(bsign)
	return sign
}
