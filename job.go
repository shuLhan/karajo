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
// automatically by timer (per interval or schedule).
//
// For job triggered by HTTP request, the Path and Secret must be set.
// For job triggered by timer, the Interval or Schedule must not be empty.
// See the [JobBase]'s Interval and Schedule fields for more information.
//
// Each Job contains a working directory, and a callback or list of commands
// to be executed.
type Job struct {
	// Shared Environment.
	env *Environment `json:"-"`

	startq chan struct{}
	stopq  chan struct{}

	// Call define a function or method to be called, as an
	// alternative to Commands.
	// This field is optional, it is only used if Job created through
	// code.
	Call JobHttpHandler `ini:"-" json:"-"`

	// HTTP path where Job can be triggered using HTTP.
	// The Path is automatically prefixed with "/karajo/api/job/run", it
	// is not static.
	// For example, if it set to "/my", then the actual path would be
	// "/karajo/api/job/run/my".
	// This field is optional and unique between Job.
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
	// If its empty, it will be set to global Secret from Environment.
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
	var (
		logp = `init`
	)

	job.JobBase.init(name)

	job.startq = make(chan struct{}, 1)
	job.stopq = make(chan struct{}, 1)

	job.Path = strings.TrimSpace(job.Path)
	job.Secret = strings.TrimSpace(job.Secret)
	if len(job.Secret) == 0 {
		job.Secret = env.Secret
	}

	if len(job.Commands) == 0 && job.Call == nil {
		return ErrJobEmptyCommandsOrCall
	}

	job.env = env

	err = job.initDirsState(env)
	if err != nil {
		return err
	}

	err = job.initLogs()
	if err != nil {
		return err
	}

	err = job.JobBase.initTimer()
	if err != nil {
		return fmt.Errorf(`%s: %w`, logp, err)
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

// handleHttp handle trigger to run the Job from HTTP request.
//
// Once the signature is verified it will response immediately and run the
// actual process in the new goroutine.
func (job *Job) handleHttp(epr *libhttp.EndpointRequest) (resbody []byte, err error) {
	var logp = `handleHttp`

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
	case job.startq <- struct{}{}:
		res.Code = http.StatusAccepted
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

// Start the Job timer only if its Interval is non-zero.
func (job *Job) Start() {
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

// startQueue start Job queue that triggered only by HTTP request.
func (job *Job) startQueue() {
	var (
		jlog *JobLog
		err  error
	)

	for {
		select {
		case <-job.startq:
			err = job.start()
			if err != nil {
				mlog.Errf(`!!! job: %s: %s`, job.ID, err)
				continue
			}

			jlog, err = job.execute(nil)
			if err != nil {
				mlog.Errf(`!!! job: %s: failed: %s.`, job.ID, err)
			} else {
				mlog.Outf(`job: %s: finished.`, job.ID)
			}
			job.finish(jlog, err)

			select {
			case job.finishq <- struct{}{}:
			default:
			}

		case <-job.stopq:
			return
		}
	}
}

func (job *Job) startScheduler() {
	var (
		jlog *JobLog
		err  error
	)

	for {
		select {
		case <-job.scheduler.C:
			select {
			case job.startq <- struct{}{}:
			default:
			}

		case <-job.startq:
			err = job.start()
			if err != nil {
				mlog.Errf(`!!! job: %s: %s`, job.ID, err)
				continue
			}

			jlog, err = job.execute(nil)
			if err != nil {
				mlog.Errf(`!!! job: %s: failed: %s.`, job.ID, err)
			} else {
				mlog.Outf(`job: %s: finished.`, job.ID)
			}
			job.finish(jlog, err)

			select {
			case job.finishq <- struct{}{}:
			default:
			}

		case <-job.stopq:
			job.scheduler.Stop()
			return
		}
	}
}

func (job *Job) startInterval() {
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

		mlog.Outf(`job: %s: next running in %s.`, job.ID, nextInterval)

		timer = time.NewTimer(nextInterval)
		ever = true
		for ever {
			select {
			case <-timer.C:
				select {
				case job.startq <- struct{}{}:
				default:
				}

			case <-job.startq:
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

				timer.Stop()
				ever = false

				select {
				case job.finishq <- struct{}{}:
				default:
				}

			case <-job.stopq:
				timer.Stop()
				return
			}
		}
	}
}

// execute the job Call or commands.
func (job *Job) execute(epr *libhttp.EndpointRequest) (jlog *JobLog, err error) {
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

	var (
		now     time.Time
		execCmd exec.Cmd
		logTime string
		cmd     string
		x       int
	)

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
	var (
		signer hash.Hash = hmac.New(sha1.New, secret)
		bsign  []byte
	)
	_, _ = signer.Write(payload)
	bsign = signer.Sum(nil)
	sign = hex.EncodeToString(bsign)
	return sign
}
