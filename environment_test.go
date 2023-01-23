// SPDX-FileCopyrightText: 2021 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

package karajo

import (
	"encoding/json"
	"testing"

	"github.com/shuLhan/share/lib/test"
)

func TestLoadEnvironment(t *testing.T) {
	var (
		tdata *test.Data
		env   *Environment
		got   []byte
		exp   []byte
		err   error
	)

	tdata, err = test.LoadData(`testdata/environment_test.txt`)
	if err != nil {
		t.Fatal(err)
	}

	env, err = LoadEnvironment("testdata/karajo.conf")
	if err != nil {
		t.Fatal(err)
	}

	err = env.initDirs()
	if err != nil {
		t.Fatal(err)
	}

	err = env.loadJobd()
	if err != nil {
		t.Fatal(err)
	}

	err = env.loadJobHttpd()
	if err != nil {
		t.Fatal(err)
	}

	got, err = json.MarshalIndent(env, ``, `  `)
	if err != nil {
		t.Fatal(err)
	}

	exp = tdata.Output[`environment.json`]

	test.Assert(t, `LoadEnvironment`, string(exp), string(got))
}

func TestEnvironment_loadJobs(t *testing.T) {
	var (
		env = &Environment{
			dirConfigJobd: `testdata/etc/karajo/job.d`,
		}
		expJobs = map[string]*Job{
			`test success`: &Job{
				Path:   `/test-success`,
				Secret: `s3cret`,
				Commands: []string{
					`echo Test job success`,
					`echo Counter is $KARAJO_JOB_COUNTER`,
					`x=$(($RANDOM%10)) && echo sleep in ${x}s && sleep $x`,
				},
			},
		}

		err error
	)

	err = env.loadJobd()
	if err != nil {
		t.Fatal(err)
	}

	test.Assert(t, `loadJobs`, expJobs, env.Jobs)
}
