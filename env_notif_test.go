// SPDX-FileCopyrightText: 2023 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

package karajo

import (
	"testing"

	"github.com/shuLhan/share/lib/ini"
	"github.com/shuLhan/share/lib/test"
)

func TestEnvNotif_ParseEnvironment(t *testing.T) {
	var (
		tdata *test.Data
		env   *Environment
		err   error
	)

	tdata, err = test.LoadData(`testdata/env_notif_test.txt`)
	if err != nil {
		t.Fatal(err)
	}

	env, err = ParseEnvironment(tdata.Input[`karajo.conf`])
	if err != nil {
		t.Fatal(err)
	}

	var (
		expRawEnv = tdata.Output[`karajo.conf.out`]
		gotRawEnv []byte
	)

	gotRawEnv, err = ini.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}

	test.Assert(t, `Notif: ParseEnvironment`, string(expRawEnv), string(gotRawEnv))
}
