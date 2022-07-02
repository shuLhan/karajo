// SPDX-FileCopyrightText: 2022 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

package karajo

import (
	"bytes"
	"strconv"
)

// hookState store the current state of a hook.
type hookState struct {
	logCounter int
}

// pack convert the state into text, each field from top to bottom
// separated by new line.
func (state *hookState) pack() (text []byte, err error) {
	var (
		buf bytes.Buffer
	)

	buf.WriteString(strconv.Itoa(state.logCounter))
	buf.WriteByte('\n')

	return buf.Bytes(), nil
}

// unpack load the state.
func (state *hookState) unpack(text []byte) (err error) {
	var (
		fields [][]byte = bytes.Split(text, []byte("\n"))
	)
	if len(fields) == 0 {
		return nil
	}
	state.logCounter, err = strconv.Atoi(string(fields[0]))
	if err != nil {
		return err
	}
	return nil
}
