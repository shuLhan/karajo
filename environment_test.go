// SPDX-FileCopyrightText: 2021 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

package karajo

import (
	"testing"
	"time"

	"github.com/shuLhan/share/lib/test"
)

func TestLoadEnvironment(t *testing.T) {
	var (
		expEnv = &Environment{
			Name:           "My karajo",
			ListenAddress:  "127.0.0.1:31937",
			HttpTimeout:    time.Duration(5 * time.Minute),
			MaxHookRunning: 2,
			Secret:         "s3cret",
			DirBase:        "testdata",
			DirPublic:      "testdata",
			file:           "testdata/karajo.conf",
			Hooks: map[string]*Hook{
				"test fail": &Hook{
					Path:   "/test-fail",
					Secret: "s3cret",
					Commands: []string{
						`echo Test hook fail`,
						`echo Counter is $KARAJO_HOOK_COUNTER`,
						`x=$(($RANDOM%10)) && echo sleep in ${x}s && sleep $x`,
						`command-not-found`,
					},
				},
				"test random": &Hook{
					Description: "Test running command with random exit status",
					Path:        "/test-random",
					Secret:      "s3cret",
					Commands: []string{
						`echo Test hook random`,
						`echo Counter is $KARAJO_HOOK_COUNTER`,
						`x=$(($RANDOM%10)) && echo sleep in ${x}s && sleep $x`,
						`rand=$(($RANDOM%2)) && echo $rand && exit $rand`,
					},
				},
				"test success": &Hook{
					Path:   "/test-success",
					Secret: "s3cret",
					Commands: []string{
						`echo Test hook success`,
						`echo Counter is $KARAJO_HOOK_COUNTER`,
						`x=$(($RANDOM%10)) && echo sleep in ${x}s && sleep $x`,
					},
				},
				`Test long running`: &Hook{
					Description: `The hook to test log refresh.`,
					Path:        `/test-long-running`,
					Secret:      `s3cret`,
					Commands: []string{
						`for ((x=0; x<90; x++)); do echo "$x"; sleep 1; done`,
					},
				},
				`Test manual run`: &Hook{
					Description: `The hook to test manual run.`,
					Path:        `/test-manual-run`,
					Secret:      `s3cret`,
					Commands: []string{
						`echo Test hook manual run`,
						`echo Counter is $KARAJO_HOOK_COUNTER`,
					},
				},
			},
			HttpJobs: map[string]*JobHttp{
				"Test fail": &JobHttp{
					Description:     "The job to test what the user interface and logs look likes if its <b>fail</b>.",
					Secret:          "s3cret",
					Interval:        time.Duration(20 * time.Second),
					MaxRequests:     2,
					HttpMethod:      "POST",
					HttpUrl:         "http://127.0.0.1:31937/karajo/hook/test-fail",
					HttpRequestType: "json",
					HttpHeaders: []string{
						"A: B",
						"C: D",
					},
				},
				"Test random": &JobHttp{
					Description:     `Test triggering hook /test-random`,
					Secret:          "s3cret",
					MaxRequests:     1,
					HttpMethod:      "POST",
					HttpUrl:         "/karajo/hook/test-random",
					HttpRequestType: "json",
				},
				"Test success": &JobHttp{
					Description:     "The job to test what the user interface and logs look likes if its <i>success</i>.",
					Secret:          "s3cret",
					Interval:        time.Duration(20 * time.Second),
					MaxRequests:     1,
					HttpMethod:      "POST",
					HttpUrl:         "/karajo/hook/test-success",
					HttpRequestType: "json",
					HttpHeaders: []string{
						"X: Y",
					},
				},
				`Test long running`: &JobHttp{
					Description:     `The job to test hook log refresh.`,
					Secret:          `s3cret`,
					Interval:        time.Duration(2 * time.Minute),
					MaxRequests:     1,
					HttpMethod:      `POST`,
					HttpUrl:         `/karajo/hook/test-long-running`,
					HttpRequestType: `json`,
				},
			},
		}

		env *Environment
		err error
	)

	env, err = LoadEnvironment("testdata/karajo.conf")
	if err != nil {
		t.Fatal(err)
	}

	test.Assert(t, "LoadEnvironment", expEnv, env)
}
