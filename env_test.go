// SPDX-FileCopyrightText: 2021 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

package karajo

import (
	"encoding/json"
	"testing"

	"git.sr.ht/~shulhan/pakakeh.go/lib/test"
)

func TestLoadEnv(t *testing.T) {
	var (
		tdata *test.Data
		env   *Env
		got   []byte
		exp   []byte
		err   error
	)

	tdata, err = test.LoadData(`testdata/environment_test.txt`)
	if err != nil {
		t.Fatal(err)
	}

	env, err = LoadEnv(`testdata/karajo.conf`)
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

	err = env.loadJobHTTPd()
	if err != nil {
		t.Fatal(err)
	}

	got, err = json.MarshalIndent(env, ``, `  `)
	if err != nil {
		t.Fatal(err)
	}

	exp = tdata.Output[`environment.json`]

	test.Assert(t, `LoadEnv`, string(exp), string(got))
}

func TestEnv_loadJobs(t *testing.T) {
	var (
		env = &Env{
			dirConfigJobd: `testdata/etc/karajo/job.d`,
		}
		expJobs = map[string]*JobExec{
			`Scheduler hourly 5m`: &JobExec{
				JobBase: JobBase{
					Schedule: `hourly@0,5,10,15,20,25,30,35,40,45,50,55`,
				},
				Path:   `/scheduler-hourly-5m`,
				Secret: `s3cret`,
				Commands: []string{
					`echo Test job scheduler hourly per 5m`,
				},
			},
			`Scheduler minutely`: &JobExec{
				JobBase: JobBase{
					Schedule: `minutely`,
				},
				Secret: `s3cret`,
				Path:   `/scheduler-minutely`,
				Commands: []string{
					`echo Test job scheduler per minute`,
				},
			},
			`Test auth_kind github`: &JobExec{
				AuthKind: `github`,
				Path:     `/github`,
				Secret:   `s3cret`,
				Commands: []string{
					`echo auth_kind is github`,
				},
			},
			`test success`: &JobExec{
				Path:   `/test-success`,
				Secret: `s3cret`,
				Commands: []string{
					`echo Test job success`,
					`echo Counter is $KARAJO_JOB_COUNTER`,
					`x=$(($RANDOM%10)) && echo sleep in ${x}s && sleep $x`,
				},
			},
			`notif-email-success`: &JobExec{
				JobBase: JobBase{
					Description: `Send notification when job success.`,
					NotifOnSuccess: []string{
						`email-to-shulhan`,
						`email-to-ops`,
					},
					NotifOnFailed: []string{
						`email-to-shulhan`,
					},
				},
				Path: `/notif-email-success`,
				Commands: []string{
					`echo Test email notification`,
				},
			},
		}

		err error
	)

	err = env.loadJobd()
	if err != nil {
		t.Fatal(err)
	}

	test.Assert(t, `loadJobs`, expJobs, env.ExecJobs)
}
