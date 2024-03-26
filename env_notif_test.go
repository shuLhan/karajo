// SPDX-FileCopyrightText: 2023 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

package karajo

import (
	"testing"

	"git.sr.ht/~shulhan/pakakeh.go/lib/ini"
	"git.sr.ht/~shulhan/pakakeh.go/lib/test"
)

func TestEnvNotif_ParseEnv(t *testing.T) {
	var (
		tdata *test.Data
		env   *Env
		err   error
	)

	tdata, err = test.LoadData(`testdata/env_notif_test.txt`)
	if err != nil {
		t.Fatal(err)
	}

	env, err = ParseEnv(tdata.Input[`karajo.conf`])
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

	test.Assert(t, `Notif: ParseEnv`, string(expRawEnv), string(gotRawEnv))
}
