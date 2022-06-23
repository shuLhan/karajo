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
			DirBase:       "testdata",
			file:          "testdata/karajo.conf",
			Jobs: []*Job{{
				Name:        "Test fail",
				Description: "The job to test what the user interface and logs look likes  if its <b>fail</b>.",
				Interval:    time.Duration(20 * time.Second),
				MaxRequests: 2,
				HttpUrl:     "http://127.0.0.1:31937/karajo/test/job/fail",
				HttpHeaders: []string{
					"A: B",
					"C: D",
				},
			}, {
				Name:        "Test success",
				Description: "The job to test what the user interface and logs look likes  if its <i>success</i>.",
				Interval:    time.Duration(20 * time.Second),
				MaxRequests: 1,
				HttpUrl:     "/karajo/test/job/success",
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
