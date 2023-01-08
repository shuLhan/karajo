// SPDX-FileCopyrightText: 2021 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

package karajo

import (
	"testing"
	"time"

	"github.com/shuLhan/share/lib/test"
)

func TestHttpJob_computeFirstTimer(t *testing.T) {
	type testCase struct {
		jobHttp *JobHttp
		desc    string
		exp     time.Duration
	}

	var (
		now      = time.Date(2021, 3, 6, 14, 0, 0, 0, time.UTC)
		interval = 30 * time.Second
	)

	var cases = []testCase{{
		desc: "Last run is -2*interval ",
		jobHttp: &JobHttp{
			jobState: jobState{
				LastRun: now.Add(-2 * interval),
			},
			Interval: interval,
		},
	}, {
		desc: "Last run is now",
		jobHttp: &JobHttp{
			jobState: jobState{
				LastRun: now.UTC(),
			},
			Interval: interval,
		},
		exp: interval,
	}, {
		desc: "Last run is half-interval ago",
		jobHttp: &JobHttp{
			jobState: jobState{
				LastRun: now.Add(-1 * (interval / 2)),
			},
			Interval: interval,
		},
		exp: interval / 2,
	}, {
		desc: "Last run > now?",
		jobHttp: &JobHttp{
			jobState: jobState{
				LastRun: now.Add(1 * interval),
			},
			Interval: interval,
		},
		exp: 2 * interval,
	}}

	var (
		c   testCase
		got time.Duration
	)
	for _, c = range cases {
		got = c.jobHttp.computeFirstTimer(now)
		test.Assert(t, c.desc, c.exp, got)
	}
}
