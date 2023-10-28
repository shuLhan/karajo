// SPDX-FileCopyrightText: 2022 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

package karajo

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// JobLog contains the content, status, and counter for job's log.
//
// Each log file name is using the following format:
//
//	<job.ID>.<counter>.<status>
//
// Counter is a number that unique between log, start from 1.
//
// Status can be success or fail.
// If status is missing its considered fail.
type JobLog struct {
	jobKind jobKind
	JobID   string `json:"job_id"`
	Name    string `json:"name"`
	path    string
	Status  string `json:"status,omitempty"`
	Content []byte `json:"content,omitempty"` // Only used to transfrom from/to JSON.
	content []byte
	Counter int64 `json:"counter,omitempty"`

	sync.Mutex
}

func newJobLog(job *JobBase) (jlog *JobLog) {
	jlog = &JobLog{
		jobKind: job.kind,
		JobID:   job.ID,
		Name:    fmt.Sprintf(`%s.%d`, job.ID, job.lastCounter),
		Status:  JobStatusStarted,
		Counter: job.lastCounter,
	}

	jlog.path = filepath.Join(job.dirLog, jlog.Name)

	return jlog
}

// parseJobLogName parse the log file name to unpack the name, counter, and
// status.
// If the name is not valid, the file is removed and it will return nil.
func parseJobLogName(dir, name string) (jlog *JobLog) {
	var (
		logFields []string = strings.Split(name, `.`)

		err error
	)

	jlog = &JobLog{
		Name: name,
		path: filepath.Join(dir, name),
	}

	if len(logFields) <= 1 {
		_ = os.Remove(jlog.path)
		return nil
	}

	jlog.JobID = logFields[0]

	jlog.Counter, err = strconv.ParseInt(logFields[1], 10, 64)
	if err != nil {
		_ = os.Remove(jlog.path)
		return nil
	}

	if len(logFields) == 2 {
		// No status on filename, assume it as fail.
		_ = os.Remove(jlog.path)
		return nil
	}

	jlog.Status = logFields[2]

	return jlog
}

func (jlog *JobLog) flush() (err error) {
	jlog.Lock()

	jlog.Name = jlog.Name + `.` + jlog.Status
	jlog.path = jlog.path + `.` + jlog.Status
	err = os.WriteFile(jlog.path, jlog.content, 0600)

	jlog.Unlock()
	return err
}

// load the content of log from storage.
func (jlog *JobLog) load() (err error) {
	jlog.Lock()
	if len(jlog.content) == 0 {
		jlog.content, err = os.ReadFile(jlog.path)
	}
	jlog.Unlock()
	return err
}

func (jlog *JobLog) marshalJSON() ([]byte, error) {
	jlog.Lock()

	var (
		buf     bytes.Buffer
		content = base64.StdEncoding.EncodeToString(jlog.content)
	)

	fmt.Fprintf(&buf, `{"job_id":%q,"name":%q,"status":%q,"counter":%d,"content":%q}`,
		jlog.JobID, jlog.Name, jlog.Status, jlog.Counter, content)

	jlog.Unlock()
	return buf.Bytes(), nil
}

func (jlog *JobLog) setStatus(status string) {
	jlog.Lock()
	jlog.Status = status
	jlog.Unlock()
}

func (jlog *JobLog) Write(b []byte) (n int, err error) {
	jlog.Lock()
	n = len(jlog.content)
	if n == 0 || n > 0 && jlog.content[n-1] == '\n' {
		var timestamp = TimeNow().UTC().Format(defTimeLayout)
		jlog.content = append(jlog.content, []byte(timestamp)...)
		jlog.content = append(jlog.content, ' ')
		jlog.content = append(jlog.content, []byte(jlog.jobKind)...)
		jlog.content = append(jlog.content, []byte(": ")...)
		jlog.content = append(jlog.content, []byte(jlog.JobID)...)
		jlog.content = append(jlog.content, []byte(": ")...)
	}
	jlog.content = append(jlog.content, b...)
	jlog.Unlock()
	return len(b), nil
}
