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

// JobLog contains the content, status, and counter for log.
//
// Each log file is using the following format:
// "<job.ID>.<counter>.<status>".
//
// Counter is a number that unique between log, start from 1.
//
// Status can be success or fail.
// If status is missing its considered fail.
type JobLog struct {
	JobID   string
	Name    string
	path    string
	Status  string
	Content []byte
	Counter int64

	sync.Mutex
}

func newJobLog(jobID, dirLog string, logCounter int64) (jlog *JobLog) {
	jlog = &JobLog{
		JobID:   jobID,
		Name:    fmt.Sprintf(`%s.%d`, jobID, logCounter),
		Status:  JobStatusStarted,
		Counter: logCounter,
	}

	jlog.path = filepath.Join(dirLog, jlog.Name)

	return jlog
}

// parseJobLogName parse the log file name to unpack the name, counter, and
// status.
// If the name is not valid, the file is removed and it will return nil.
func parseJobLogName(dir, name string) (jlog *JobLog) {
	var (
		logFields []string = strings.Split(name, ".")

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
	defer jlog.Unlock()

	jlog.Name = jlog.Name + `.` + jlog.Status
	jlog.path = jlog.path + `.` + jlog.Status
	err = os.WriteFile(jlog.path, jlog.Content, 0600)
	if err != nil {
		return err
	}
	return nil
}

// load the content of log from storage.
func (jlog *JobLog) load() (err error) {
	jlog.Lock()
	defer jlog.Unlock()

	if len(jlog.Content) != 0 {
		return nil
	}

	jlog.Content, err = os.ReadFile(jlog.path)
	if err != nil {
		return err
	}
	return nil
}

func (jlog *JobLog) MarshalJSON() ([]byte, error) {
	jlog.Lock()
	defer jlog.Unlock()

	var (
		buf     bytes.Buffer
		content = base64.StdEncoding.EncodeToString(jlog.Content)
	)

	fmt.Fprintf(&buf, `{"JobID":%q,"Name":%q,"Status":%q,"Counter":%d,"Content":%q}`,
		jlog.JobID, jlog.Name, jlog.Status, jlog.Counter, content)

	return buf.Bytes(), nil
}

func (jlog *JobLog) setStatus(status string) {
	jlog.Lock()
	jlog.Status = status
	jlog.Unlock()
}

func (jlog *JobLog) Write(b []byte) (n int, err error) {
	jlog.Lock()
	jlog.Content = append(jlog.Content, b...)
	jlog.Unlock()
	return len(b), nil
}
