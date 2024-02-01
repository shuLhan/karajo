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
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"

	libhttp "github.com/shuLhan/share/lib/http"
	"github.com/shuLhan/share/lib/mlog"
)

const (
	defJobLogRetention = 5

	jobEnvCounter   = `KARAJO_JOB_COUNTER`
	jobEnvPath      = `PATH`
	jobEnvPathValue = `/usr/local/sbin:/usr/local/bin:/usr/bin:/usr/bin/site_perl:/usr/bin/vendor_perl:/usr/bin/core_perl`
)

// List of [JobExec.AuthKind] for authorization.
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

// JobExecHTTPHandler define an handler for triggering a JobExec using HTTP.
//
// The log parameter is used to log all output and error.
// The epr parameter contains HTTP request, body, and response writer.
type JobExecHTTPHandler func(log io.Writer, epr *libhttp.EndpointRequest) error

// JobExec define a job to execute Go code or list of commands.
// A JobExec can be triggered manually by sending HTTP POST request or
// automatically by timer (per interval or schedule).
//
// For job triggered by HTTP request, the Path and Secret must be set.
// For job triggered by timer, the Interval or Schedule must not be empty.
// See the [JobBase]'s Interval and Schedule fields for more information.
//
// Each JobExec have its own working directory, a callback or list of
// commands to be executed.
//
// The JobExec configuration in INI format,
//
//	[job "name"]
//	path =
//	auth_kind =
//	header_sign =
//	secret =
//	command =
type JobExec struct {
	// jobq is a channel passed by Karajo instance to limit number of
	// job running at the same time.
	jobq chan struct{}

	httpq chan *libhttp.EndpointRequest
	stopq chan struct{}

	// Call define a function or method to be called, as an
	// alternative to Commands.
	// This field is optional, it is only used if JobExec created
	// through code.
	Call JobExecHTTPHandler `ini:"-" json:"-"`

	// HTTP path where JobExec can be triggered using HTTP.
	// The Path is automatically prefixed with "/karajo/api/job/run", it
	// is not static.
	// For example, if it set to "/my", then the actual path would be
	// "/karajo/api/job/run/my".
	// This field is optional and unique between JobExec.
	Path string `ini:"::path" json:"path,omitempty"`

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
	AuthKind string `ini:"::auth_kind" json:"auth_kind,omitempty"`

	// HeaderSign define the HTTP header where the signature is read.
	// Default to "X-Karajo-Sign" if its empty.
	HeaderSign string `ini:"::header_sign" json:"header_sign,omitempty"`

	// Secret define a string to validate the signature of request.
	// If its empty, it will be set to global Secret from Env.
	Secret string `ini:"::secret" json:"-"`

	// Commands list of command to be executed.
	// This option can be defined multiple times.
	// The following environment variables are available inside the
	// command:
	//
	//   - KARAJO_JOB_COUNTER: contains the current job counter.
	Commands []string `ini:"::command" json:"commands,omitempty"`

	JobBase
}

// authorize the hook based on the AuthKind.
func (job *JobExec) authorize(headers http.Header, reqbody []byte) (err error) {
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
func (job *JobExec) authGithub(headers http.Header, reqbody []byte) (err error) {
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
func (job *JobExec) authSourcehut(headers http.Header, reqbody []byte, pubkey ed25519.PublicKey) (err error) {
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
func (job *JobExec) authHmacSha256(headers http.Header, reqbody []byte) (err error) {
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

func (job *JobExec) generateCmdEnvs() (env []string) {
	env = append(env, fmt.Sprintf(`%s=%d`, jobEnvCounter, job.counter))
	env = append(env, fmt.Sprintf(`%s=%s`, jobEnvPath, jobEnvPathValue))
	return env
}

// init initialize the JobExec.
//
// For JobExec that need to be triggered by HTTP request the Path and Secret
// _must_ not be empty.
// If Secret is not set then it will default to Env's Secret.
//
// It will return an error ErrJobEmptyCommandsOrCall if one of the Call or
// Commands is not set.
func (job *JobExec) init(env *Env, name string) (err error) {
	var (
		logp = `init`
	)

	job.JobBase.kind = jobKindExec

	err = job.JobBase.init(env, name)
	if err != nil {
		return fmt.Errorf(`%s: %w`, logp, err)
	}

	job.httpq = make(chan *libhttp.EndpointRequest, 1)
	job.stopq = make(chan struct{}, 1)

	job.Path = strings.TrimSpace(job.Path)
	job.Secret = strings.TrimSpace(job.Secret)
	if len(job.Secret) == 0 {
		job.Secret = env.Secret
	}

	if len(job.Commands) == 0 && job.Call == nil {
		return ErrJobEmptyCommandsOrCall
	}

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

// handleHTTP trigger running the JobExec by HTTP request.
//
// Once the signature is verified it will response immediately and run the
// actual process in the new goroutine.
func (job *JobExec) handleHTTP(epr *libhttp.EndpointRequest) (resbody []byte, err error) {
	var logp = `handleHTTP`

	// Authenticated request by checking the request body.
	err = job.authorize(epr.HttpRequest.Header, epr.RequestBody)
	if err != nil {
		return nil, fmt.Errorf(`%s: %s: %w`, logp, job.ID, err)
	}

	err = job.canStart()
	if err != nil {
		return nil, fmt.Errorf(`%s: %s: %w`, logp, job.ID, err)
	}

	var res libhttp.EndpointResponse

	select {
	case job.httpq <- epr:
		res.Code = http.StatusOK
		res.Message = `OK`
		res.Data = job
	default:
		return nil, &ErrJobAlreadyRun
	}

	job.Lock()
	resbody, err = json.Marshal(&res)
	job.Unlock()

	return resbody, err
}

// Start the job queue, either by scheduler, interval, or waiting for
// request.
func (job *JobExec) Start(jobq chan struct{}, logq chan<- *JobLog) {
	job.jobq = jobq
	job.JobBase.logq = logq

	if job.scheduler != nil {
		job.startScheduler()
		return
	}
	if job.Interval > 0 {
		job.startInterval()
		return
	}
	job.startQueue()
}

// startQueue start JobExec queue that triggered only by HTTP request.
func (job *JobExec) startQueue() {
	var epr *libhttp.EndpointRequest

	for {
		select {
		case epr = <-job.httpq:
			job.run(epr)

		case <-job.stopq:
			return
		}
	}
}

func (job *JobExec) startScheduler() {
	var epr *libhttp.EndpointRequest

	for {
		select {
		case <-job.scheduler.C:
			epr = nil

		case epr = <-job.httpq:
			// Job triggered by HTTP request.

		case <-job.stopq:
			job.scheduler.Stop()
			return
		}

		job.run(epr)
	}
}

func (job *JobExec) startInterval() {
	var (
		now          time.Time
		nextInterval time.Duration
		timer        *time.Timer
		epr          *libhttp.EndpointRequest
	)

	for {
		job.Lock()
		now = timeNow()
		nextInterval = job.computeNextInterval(now)
		job.NextRun = now.Add(nextInterval)
		job.Unlock()

		if timer == nil {
			timer = time.NewTimer(nextInterval)
		} else {
			timer.Reset(nextInterval)
		}

		select {
		case <-timer.C:
			epr = nil

		case epr = <-job.httpq:
			// Job is triggered by HTTP request.
			timer.Stop()

		case <-job.stopq:
			timer.Stop()
			return
		}
		job.run(epr)
	}
}

func (job *JobExec) run(epr *libhttp.EndpointRequest) {
	var (
		jlog *JobLog
		err  error
	)

	job.jobq <- struct{}{}
	jlog, err = job.execute(epr)
	<-job.jobq

	job.finish(jlog, err)
}

// execute the job Call or Commands.
func (job *JobExec) execute(epr *libhttp.EndpointRequest) (jlog *JobLog, err error) {
	jlog = job.JobBase.newLog()
	if jlog.Status == JobStatusPaused {
		return jlog, nil
	}

	_, _ = jlog.Write([]byte("=== BEGIN\n"))

	// Call the job.
	if job.Call != nil {
		err = job.Call(jlog, epr)
		return jlog, err
	}

	var (
		execCmd exec.Cmd
		cmd     string
		x       int
	)

	// Run commands.
	for x, cmd = range job.Commands {
		_, _ = jlog.Write([]byte("\n"))
		fmt.Fprintf(jlog, "--- Execute %2d: %s\n", x, cmd)

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

	_, _ = jlog.Write([]byte("=== DONE\n"))

	return jlog, nil
}

// Stop the JobExec queue.
func (job *JobExec) Stop() {
	mlog.Outf(`job: %s: stopping ...`, job.ID)

	select {
	case job.stopq <- struct{}{}:
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
	var signer = hmac.New(sha1.New, secret)

	_, _ = signer.Write(payload)
	var bsign = signer.Sum(nil)
	sign = hex.EncodeToString(bsign)
	return sign
}
