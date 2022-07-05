// SPDX-FileCopyrightText: 2022 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

package karajo

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// HookLog contains the content, status, and counter for log.
//
// Each log file is using the following format:
// "<hook.ID>.<counter>.<status>".
//
// Counter is a number that unique between log, start from 1.
//
// Status can be success or fail.
// If status is missing its considered fail.
type HookLog struct {
	HookID  string
	Name    string
	path    string
	Status  string
	Content []byte
	Counter int64
}

func newHookLog(hookID, dirLog string, logCounter int64) (hlog *HookLog) {
	hlog = &HookLog{
		HookID:  hookID,
		Name:    fmt.Sprintf("%s.%d", hookID, logCounter),
		Counter: logCounter,
	}

	hlog.path = filepath.Join(dirLog, hlog.Name)

	return hlog
}

// parseHookLogName parse the log file name to unpack the name, counter, and
// status.
// If the name is not valid, the file is removed and it will return nil.
func parseHookLogName(dir, name string) (hlog *HookLog) {
	var (
		logFields []string = strings.Split(name, ".")

		err error
	)

	hlog = &HookLog{
		Name: name,
		path: filepath.Join(dir, name),
	}

	if len(logFields) <= 1 {
		_ = os.Remove(hlog.path)
		return nil
	}

	hlog.HookID = logFields[0]

	hlog.Counter, err = strconv.ParseInt(logFields[1], 10, 64)
	if err != nil {
		_ = os.Remove(hlog.path)
		return nil
	}

	if len(logFields) == 2 {
		// No status on filename, assume it as fail.
		_ = os.Remove(hlog.path)
		return nil
	}

	hlog.Status = logFields[2]

	return hlog
}

func (hlog *HookLog) flush() (err error) {
	hlog.Name = hlog.Name + "." + hlog.Status
	hlog.path = hlog.path + "." + hlog.Status
	err = os.WriteFile(hlog.path, hlog.Content, 0600)
	if err != nil {
		return err
	}
	return nil
}

// load the content of log from storage.
func (hlog *HookLog) load() (err error) {
	hlog.Content, err = os.ReadFile(hlog.path)
	if err != nil {
		return err
	}
	return nil
}

func (hlog *HookLog) Write(b []byte) (n int, err error) {
	hlog.Content = append(hlog.Content, b...)
	return len(b), nil
}
