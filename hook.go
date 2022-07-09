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
	ErrHookInvalidPath = liberrors.E{
		Code:    http.StatusBadRequest,
		Name:    "ERR_HOOK_INVALID_PATH",
		Message: "invalid or empty Hook Path",
	}
	ErrHookInvalidSecret = liberrors.E{
		Code:    http.StatusBadRequest,
		Name:    "ERR_HOOK_INVALID_SECRET",
		Message: "invalid or empty secret",
	}
)

const (
	defHookLogRetention = 5

	hookEnvCounter = "KARAJO_HOOK_COUNTER"
)

// HookHandler define a function signature for handling Hook using code.
// The log parameter should be used to log all output and error.
// The epr parameter contains HTTP request, body, and response writer.
type HookHandler func(log io.Writer, epr *libhttp.EndpointRequest) error

// Hook is HTTP endpoint inside the Karajo that can be triggered from
// external using POST method.
//
// Each Hook contains Secret for authenticating request, a working directory,
// and a callback or list of commands to be executed when the request
// received.
type Hook struct {
	// Cache of log sorted by its counter.
	Logs []*HookLog

	// Call define a function or method to be called, as an
	// alternative to Commands.
	// This field is optional, it is only used if Hook created through
	// code.
	Call HookHandler `json:"-" ini:"-"`

	// The id of the hook.
	// It is normalized from the Name.
	ID string `ini:"-"`

	Name string `ini:"-"`

	// The description of the hook.
	// It could be plain text or simple HTML.
	Description string `ini:"description"`

	// HTTP path where Karajo will listen for request.
	// The Path is automatically prefixed with "/karajo/hook", it is not
	// static.
	// For example, if it set to "/my", then the actual path would be
	// "/karajo/hook/my".
	// This field is required and unique between Hook.
	Path string `ini:"::path"`

	// Secret define a string to check signature of request.
	// Each request sign the body with HMAC + SHA-256 using this secret.
	// The signature then sent in HTTP header "X-Karajo-Sign" as hex.
	// This field is required.
	Secret string `ini:"::secret" json:"-"`

	// dirWork define the directory on the system where all commands
	// will be executed.
	dirWork string
	dirLog  string

	LastStatus string // The last status of hook.

	// Commands list of command to be executed.
	Commands []string `ini:"::command"`

	LogRetention int `ini:"::log_retention"`
	lastCounter  int64

	sync.Mutex
}

func (hook *Hook) generateCmdEnvs() (env []string) {
	env = append(env, fmt.Sprintf("%s=%d", hookEnvCounter, hook.lastCounter))
	return env
}

func (hook *Hook) init(env *Environment, name string) (err error) {
	hook.Path = strings.TrimSpace(hook.Path)
	if hook.Path == "" {
		return &ErrHookInvalidPath
	}

	hook.Secret = strings.TrimSpace(hook.Secret)
	if hook.Secret == "" {
		return &ErrHookInvalidSecret
	}

	if len(hook.Commands) == 0 {
		if hook.Call == nil {
			return &ErrHookEmptyCommandsOrCall
		}
	}

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
		dir  *os.File
		hlog *HookLog
		fi   os.FileInfo
		fis  []os.FileInfo
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
			hook.LastStatus = hlog.Status
		}
	}

	sort.Slice(hook.Logs, func(x, y int) bool {
		return hook.Logs[x].Counter < hook.Logs[y].Counter
	})

	hook.logsPrune()

	return nil
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

func (hook *Hook) run(epr *libhttp.EndpointRequest) (resbody []byte, err error) {
	var (
		execCmd exec.Cmd
		hlog    *HookLog
		cmd     string
		expSign string
		gotSign string
		x       int
	)

	// Authenticated request by checking the request body.
	gotSign = epr.HttpRequest.Header.Get(HeaderNameXKarajoSign)
	if len(gotSign) == 0 {
		return nil, &ErrHookForbidden
	}

	mlog.Outf("%s: request body: %s", epr.HttpRequest.URL, epr.RequestBody)

	expSign = Sign(epr.RequestBody, []byte(hook.Secret))
	if expSign != gotSign {
		mlog.Outf("Sign: exp %s got %s", expSign, gotSign)
		return nil, &ErrHookForbidden
	}

	hook.Lock()
	defer hook.Unlock()

	hook.lastCounter++
	hlog = newHookLog(hook.ID, hook.dirLog, hook.lastCounter)

	hook.LastStatus = JobStatusSuccess

	// Call the hook.
	if hook.Call != nil {
		err = hook.Call(hlog, epr)
		if err != nil {
			hook.LastStatus = JobStatusFailed
		}
		return hook.writeResponse(epr, hlog, err)
	}

	// Run commands.
	for x, cmd = range hook.Commands {
		fmt.Fprintf(hlog, "\n=== Execute %2d: %s\n", x, cmd)

		execCmd = exec.Cmd{
			Path: "/bin/sh",
			Dir:  hook.dirWork,
			Args: []string{
				"/bin/sh",
				"-c",
				cmd,
			},
			Env:    hook.generateCmdEnvs(),
			Stdout: hlog,
			Stderr: hlog,
		}

		err = execCmd.Run()
		if err != nil {
			hook.LastStatus = JobStatusFailed
			return hook.writeResponse(epr, hlog, err)
		}
	}

	return hook.writeResponse(epr, hlog, nil)
}

func (hook *Hook) writeResponse(epr *libhttp.EndpointRequest, hlog *HookLog, err error) ([]byte, error) {
	var (
		res = libhttp.EndpointResponse{
			Data: hlog,
		}

		e  *liberrors.E
		ok bool
	)

	if err != nil {
		hlog.Status = JobStatusFailed
		mlog.Errf("hook: %s: %s", hook.Path, err)

		e, ok = err.(*liberrors.E)
		if !ok {
			res.Code = http.StatusInternalServerError
			res.Message = err.Error()
		} else {
			res.E = *e
		}
	} else {
		hlog.Status = JobStatusSuccess
		res.Code = http.StatusOK
	}

	hook.Logs = append(hook.Logs, hlog)
	hook.logsPrune()

	err = hlog.flush()
	if err != nil {
		mlog.Errf("hook: %s: %s", hook.Path, err)
	}

	epr.HttpWriter.WriteHeader(res.Code)

	return json.Marshal(&res)
}
