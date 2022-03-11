// SPDX-FileCopyrightText: 2021 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

package karajo

import (
	"testing"
	"time"

	"github.com/shuLhan/share/lib/test"
)

func TestJob_computeFirstTimer(t *testing.T) {
	now := time.Date(2021, 3, 6, 14, 0, 0, 0, time.UTC)
	interval := 30 * time.Second

	cases := []struct {
		desc string
		job  Job
		exp  time.Duration
	}{{
		desc: "Last run is -2*interval ",
		job: Job{
			LastRun:  now.Add(-2 * interval),
			Interval: interval,
		},
	}, {
		desc: "Last run is now",
		job: Job{
			LastRun:  now.UTC(),
			Interval: interval,
		},
		exp: interval,
	}, {
		desc: "Last run is half-interval ago",
		job: Job{
			LastRun:  now.Add(-1 * (interval / 2)),
			Interval: interval,
		},
		exp: interval / 2,
	}, {
		desc: "Last run > now?",
		job: Job{
			LastRun:  now.Add(1 * interval),
			Interval: interval,
		},
		exp: 2 * interval,
	}}

	for _, c := range cases {
		got := c.job.computeFirstTimer(now)
		test.Assert(t, c.desc, c.exp, got)
	}
}
