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
			Name:          "My karajo",
			ListenAddress: "127.0.0.1:31937",
			HttpTimeout:   time.Duration(5 * time.Minute),
			Secret:        "s3cret",
			DirBase:       "testdata",
			file:          "testdata/karajo.conf",
			Hooks: map[string]*Hook{
				"test fail": &Hook{
					Path:   "/test-fail",
					Secret: "s3cret",
					Commands: []string{
						`echo Test hook fail`,
						`command-not-found`,
					},
				},
				"test random": &Hook{
					Description: "Test running command with random exit status",
					Path:        "/test-random",
					Secret:      "s3cret",
					Commands: []string{
						`rand=$(($RANDOM%2)) && echo $rand && exit $rand`,
					},
				},
				"test success": &Hook{
					Path:   "/test-success",
					Secret: "s3cret",
					Commands: []string{
						`echo Test hook success`,
					},
				},
			},
			Jobs: []*Job{{
				Name:            "Test fail",
				Description:     "The job to test what the user interface and logs look likes  if its <b>fail</b>.",
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
			}, {
				Name:            "Test random",
				Description:     `Test triggering hook /test-random`,
				Secret:          "s3cret",
				MaxRequests:     1,
				HttpMethod:      "POST",
				HttpUrl:         "/karajo/hook/test-random",
				HttpRequestType: "json",
			}, {
				Name:            "Test success",
				Description:     "The job to test what the user interface and logs look likes  if its <i>success</i>.",
				Secret:          "s3cret",
				Interval:        time.Duration(20 * time.Second),
				MaxRequests:     1,
				HttpMethod:      "POST",
				HttpUrl:         "/karajo/hook/test-success",
				HttpRequestType: "json",
				HttpHeaders: []string{
					"X: Y",
				},
			}},
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
