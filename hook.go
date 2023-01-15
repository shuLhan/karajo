// SPDX-FileCopyrightText: 2022 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

package karajo

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	liberrors "github.com/shuLhan/share/lib/errors"
	libhttp "github.com/shuLhan/share/lib/http"
	"github.com/shuLhan/share/lib/mlog"
	libhtml "github.com/shuLhan/share/lib/net/html"
)

// List of errors.
var (
	ErrHookEmptyCommandsOrCall = liberrors.E{
		Code:    http.StatusBadRequest,
		Name:    "ERR_HOOK_EMPTY_COMMANDS_OR_CALL",
		Message: "empty commands or call handle",
	}
	ErrHookForbidden = liberrors.E{
		Code:    http.StatusForbidden,
		Name:    "ERR_HOOK_FORBIDDEN",
		Message: "forbidden",
	}
	ErrHookInvalidSecret = liberrors.E{
		Code:    http.StatusBadRequest,
		Name:    "ERR_HOOK_INVALID_SECRET",
		Message: "invalid or empty secret",
	}
)

const (
	defHookLogRetention = 5

	hookEnvCounter   = "KARAJO_HOOK_COUNTER"
	hookEnvPath      = "PATH"
	hookEnvPathValue = "/usr/local/sbin:/usr/local/bin:/usr/bin:/usr/bin/site_perl:/usr/bin/vendor_perl:/usr/bin/core_perl"
)

// HookHandler define a function signature for handling Hook using code.
// The log parameter should be used to log all output and error.
// The epr parameter contains HTTP request, body, and response writer.
type HookHandler func(log io.Writer, epr *libhttp.EndpointRequest) error

// Hook is a job that can be triggered manually by sending HTTP POST request
// or automatically by timer (per interval).
//
// For hook triggered by HTTP request, the Path and Secret must be set.
// For hook triggered by timer, the Interval must be positive duration, equal
// or greater than 1 minute.
//
// Each Hook contains a working directory, and a callback or list of commands
// to be executed.
type Hook struct {
	// Shared Environment.
	env *Environment `json:"-"`

	// Cache of log sorted by its counter.
	Logs []*HookLog

	// Call define a function or method to be called, as an
	// alternative to Commands.
	// This field is optional, it is only used if Hook created through
	// code.
	Call HookHandler `json:"-" ini:"-"`

	// HTTP path where Karajo will listen for request.
	// The Path is automatically prefixed with "/karajo/hook", it is not
	// static.
	// For example, if it set to "/my", then the actual path would be
	// "/karajo/hook/my".
	// This field is required and unique between Hook.
	Path string `ini:"::path"`

	// HeaderSign define the HTTP header where the signature is read.
	// Default to "x-karajo-sign" if its empty.
	HeaderSign string `ini:"::header_sign"`

	// Secret define a string to check signature of request.
	// Each request sign the body with HMAC + SHA-256 using this secret.
	// The signature then sent in HTTP header "X-Karajo-Sign" as hex.
	// This field is required if Path is not empty.
	Secret string `ini:"::secret" json:"-"`

	// dirWork define the directory on the system where all commands
	// will be executed.
	dirWork string
	dirLog  string

	// Commands list of command to be executed.
	Commands []string `ini:"::command"`

	JobBase

	LogRetention int `ini:"::log_retention"`
	lastCounter  int64

	sync.Mutex
}

// finish mark the hook as finished with status.
func (hook *Hook) finish(hlog *HookLog, status string) {
	var (
		err error
	)

	if hlog != nil {
		hlog.setStatus(status)
		err = hlog.flush()
		if err != nil {
			mlog.Errf("hook: %s: %s", hook.ID, err)
		}
	}

	hook.Lock()
	hook.NumRunning--
	hook.LastRun = TimeNow().UTC().Round(time.Second)
	hook.Status = status
	if hook.Interval > 0 {
		hook.NextRun = hook.LastRun.Add(hook.Interval)
	}

	select {
	case hook.finished <- true:
	default:
	}
	hook.Unlock()

	mlog.Outf("hook: %s: %s", hook.ID, status)
}

func (hook *Hook) generateCmdEnvs() (env []string) {
	env = append(env, fmt.Sprintf("%s=%d", hookEnvCounter, hook.lastCounter))
	env = append(env, fmt.Sprintf("%s=%s", hookEnvPath, hookEnvPathValue))
	return env
}

// init initialize the Hook.
//
// For Hook that need to be triggered by HTTP request the Path and Secret
// _must_ not be empty, otherwise it will return an error
// ErrHookInvalidSecret.
//
// It will return an error ErrHookEmptyCommandsOrCall if one of the Call or
// Commands is not set.
func (hook *Hook) init(env *Environment, name string) (err error) {
	hook.JobBase.init()

	hook.Path = strings.TrimSpace(hook.Path)
	hook.Secret = strings.TrimSpace(hook.Secret)
	if len(hook.Path) != 0 && len(hook.Secret) == 0 {
		return &ErrHookInvalidSecret
	}

	if len(hook.Commands) == 0 && hook.Call == nil {
		return &ErrHookEmptyCommandsOrCall
	}

	hook.env = env
	hook.Name = name
	hook.ID = libhtml.NormalizeForID(name)
	if hook.LogRetention <= 0 {
		hook.LogRetention = defHookLogRetention
	}

	err = hook.initDirsState(env)
	if err != nil {
		return err
	}

	err = hook.initLogs()
	if err != nil {
		return err
	}

	hook.initTimer()

	if len(hook.HeaderSign) == 0 {
		hook.HeaderSign = HeaderNameXKarajoSign
	}

	return nil
}

func (hook *Hook) initDirsState(env *Environment) (err error) {
	hook.dirWork = filepath.Join(env.dirLibHook, hook.ID)
	err = os.MkdirAll(hook.dirWork, 0700)
	if err != nil {
		return err
	}

	hook.dirLog = filepath.Join(env.dirLogHook, hook.ID)
	err = os.MkdirAll(hook.dirLog, 0700)
	if err != nil {
		return err
	}

	return nil
}

// initLogs load the hook logs state, counter and status.
func (hook *Hook) initLogs() (err error) {
	var (
		dir       *os.File
		hlog      *HookLog
		fi        os.FileInfo
		fiModTime time.Time
		fis       []os.FileInfo
	)

	dir, err = os.Open(hook.dirLog)
	if err != nil {
		return err
	}
	fis, err = dir.Readdir(0)
	if err != nil {
		return err
	}

	for _, fi = range fis {
		hlog = parseHookLogName(hook.dirLog, fi.Name())
		if hlog == nil {
			// Skip log with invalid file name.
			continue
		}

		hook.Logs = append(hook.Logs, hlog)

		if hlog.Counter > hook.lastCounter {
			hook.lastCounter = hlog.Counter
			hook.Status = hlog.Status
		}

		fiModTime = fi.ModTime()
		if hook.LastRun.IsZero() {
			hook.LastRun = fiModTime
		} else if fiModTime.After(hook.LastRun) {
			hook.LastRun = fiModTime
		}
	}

	hook.LastRun = hook.LastRun.UTC().Round(time.Second)

	sort.Slice(hook.Logs, func(x, y int) bool {
		return hook.Logs[x].Counter < hook.Logs[y].Counter
	})

	hook.logsPrune()

	return nil
}

// initTimer init fields that required to run Hook with interval.
func (hook *Hook) initTimer() {
	if hook.Interval <= 0 {
		return
	}
	if hook.Interval < time.Minute {
		hook.Interval = time.Minute
	}
}

func (hook *Hook) logsPrune() {
	var (
		hlog     *HookLog
		totalLog int
		indexMin int
	)

	totalLog = len(hook.Logs)
	if totalLog > hook.LogRetention {
		// Delete old logs.
		indexMin = totalLog - hook.LogRetention
		for _, hlog = range hook.Logs[:indexMin] {
			_ = os.Remove(hlog.path)
		}
		hook.Logs = hook.Logs[indexMin:]
	}
}

// handleHttp handle trigger to run the Hook from HTTP request.
//
// Once the signature is verified it will response immediately and run the
// actual process in the new goroutine.
func (hook *Hook) handleHttp(epr *libhttp.EndpointRequest) (resbody []byte, err error) {
	var (
		res      libhttp.EndpointResponse
		zeroTime time.Time
		expSign  string
		gotSign  string
	)

	hook.Lock()
	hook.Status = JobStatusStarted
	hook.LastRun = zeroTime
	hook.Unlock()

	// Authenticated request by checking the request body.
	gotSign = epr.HttpRequest.Header.Get(hook.HeaderSign)
	if len(gotSign) == 0 {
		gotSign = epr.HttpRequest.Header.Get(HeaderNameXKarajoSign)
		if len(gotSign) == 0 {
			hook.finish(nil, JobStatusFailed)
			return nil, &ErrHookForbidden
		}
	}

	gotSign = strings.TrimPrefix(gotSign, "sha256=")

	expSign = Sign(epr.RequestBody, []byte(hook.Secret))
	if expSign != gotSign {
		mlog.Outf("hook: %s: expecting signature %s got %s", hook.ID, expSign, gotSign)
		hook.finish(nil, JobStatusFailed)
		return nil, &ErrHookForbidden
	}

	go hook.execute(epr)

	res.Code = http.StatusOK
	res.Message = "OK"
	res.Data = hook

	hook.Lock()
	resbody, err = json.Marshal(&res)
	hook.Unlock()

	return resbody, err
}

// Start the Hook timer only if its Interval is non-zero.
func (hook *Hook) Start() {
	if hook.Interval <= 0 {
		return
	}

	var (
		now          time.Time
		nextInterval time.Duration
		timer        *time.Timer
		ever         bool
	)

	for {
		hook.Lock()
		now = TimeNow().UTC().Round(time.Second)
		nextInterval = hook.computeNextInterval(now)
		hook.NextRun = now.Add(nextInterval)
		hook.Unlock()

		mlog.Outf("hook: %s: next running in %s ...", hook.ID, nextInterval)

		timer = time.NewTimer(nextInterval)
		ever = true
		for ever {
			select {
			case <-timer.C:
				hook.execute(nil)
				// The execute will trigger the finished
				// channel.

			case <-hook.finished:
				timer.Stop()
				ever = false

			case <-hook.stopped:
				timer.Stop()
				return
			}
		}
	}
}

// execute the hook Call or commands.
func (hook *Hook) execute(epr *libhttp.EndpointRequest) {
	var (
		hlog    *HookLog
		execCmd exec.Cmd
		now     time.Time
		cmd     string
		err     error
		x       int
	)

	hook.env.hookq <- struct{}{}
	mlog.Outf("hook: %s: started ...", hook.ID)
	defer func() {
		<-hook.env.hookq
	}()

	hook.Lock()
	hook.NumRunning++
	hook.lastCounter++
	hlog = newHookLog(hook.ID, hook.dirLog, hook.lastCounter)

	hook.Logs = append(hook.Logs, hlog)
	hook.logsPrune()
	hook.Unlock()

	// Call the hook.
	if hook.Call != nil {
		err = hook.Call(hlog, epr)
		if err != nil {
			_, _ = hlog.Write([]byte(err.Error()))
			hook.finish(hlog, JobStatusFailed)
		} else {
			hook.finish(hlog, JobStatusSuccess)
		}
		return
	}

	// Run commands.
	for x, cmd = range hook.Commands {
		now = TimeNow().UTC()
		fmt.Fprintf(hlog, "\n%s === Execute %2d: %s\n", now.Format(defTimeLayout), x, cmd)

		execCmd = exec.Cmd{
			Path:   "/bin/sh",
			Dir:    hook.dirWork,
			Args:   []string{"/bin/sh", "-c", cmd},
			Env:    hook.generateCmdEnvs(),
			Stdout: hlog,
			Stderr: hlog,
		}

		err = execCmd.Run()
		if err != nil {
			_, _ = hlog.Write([]byte(err.Error()))
			hook.finish(hlog, JobStatusFailed)
			return
		}
	}

	hook.finish(hlog, JobStatusSuccess)
}

// Stop the Hook timer execution.
func (hook *Hook) Stop() {
	mlog.Outf("hook %s: stopping ...", hook.ID)

	select {
	case hook.stopped <- true:
	default:
	}
}
