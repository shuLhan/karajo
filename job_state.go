// SPDX-FileCopyrightText: 2022 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

package karajo

import (
	"bytes"
	"time"
)

// List of job status.
const (
	JobStatusStarted = "started"
	JobStatusSuccess = "success"
	JobStatusFailed  = "failed"
	JobStatusPaused  = "paused"
)

// jobState store the last job running and its status.
// Its implement encoding's TextMarshaler and TextUnmarshaler.
type jobState struct {
	// The last time the job is running, in UTC.
	LastRun time.Time

	// The last status of the job.
	Status string
}

// pack convert the job state into text, each field from top to bottom
// separated by new line.
func (state *jobState) pack() (text []byte, err error) {
	var (
		buf bytes.Buffer
		raw []byte
	)

	raw, err = state.LastRun.MarshalText()
	if err != nil {
		return nil, err
	}
	buf.Write(raw)
	buf.WriteByte('\n')
	buf.WriteString(state.Status)
	buf.WriteByte('\n')
	return buf.Bytes(), nil
}

// unpack load the job state.
func (state *jobState) unpack(text []byte) (err error) {
	var (
		fields [][]byte = bytes.Split(text, []byte("\n"))
	)
	if len(fields) == 0 {
		return nil
	}
	err = state.LastRun.UnmarshalText(fields[0])
	if err != nil {
		return err
	}
	if len(fields) == 1 {
		return nil
	}
	state.Status = string(fields[1])
	return nil
}
