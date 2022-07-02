// SPDX-FileCopyrightText: 2022 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

package karajo

import (
	"fmt"
	"os"
	"path/filepath"
)

type hookLog struct {
	name    string
	path    string
	Log     []byte
	Counter int
}

func createHookLog(hookID, dirLog string, logCounter int) (hlog hookLog) {
	hlog.name = fmt.Sprintf("%s.%d", hookID, logCounter)
	hlog.path = filepath.Join(dirLog, hlog.name)
	hlog.Counter = logCounter

	return hlog
}

func (hlog *hookLog) flush() (err error) {
	err = os.WriteFile(hlog.path, hlog.Log, 0600)
	if err != nil {
		return err
	}
	return nil
}

func (hlog *hookLog) Write(b []byte) (n int, err error) {
	hlog.Log = append(hlog.Log, b...)
	return len(b), nil
}
