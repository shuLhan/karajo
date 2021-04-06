// Copyright 2021, Shulhan <ms@kilabit.info>. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package karajo

import (
	"testing"
	"time"

	"github.com/shuLhan/share/lib/test"
)

func TestJob_computeFirstTimer(t *testing.T) {
	now := time.Date(2021, 3, 6, 14, 0, 0, 0, time.UTC)
	delay := 30 * time.Second

	cases := []struct {
		desc string
		job  Job
		exp  time.Duration
	}{{
		desc: "Last run is -2*delay ",
		job: Job{
			LastRun: now.Add(-2 * delay),
			Delay:   delay,
		},
	}, {
		desc: "Last run is now",
		job: Job{
			LastRun: now.UTC(),
			Delay:   delay,
		},
		exp: delay,
	}, {
		desc: "Last run is half-delay ago",
		job: Job{
			LastRun: now.Add(-1 * (delay / 2)),
			Delay:   delay,
		},
		exp: delay / 2,
	}, {
		desc: "Last run > now?",
		job: Job{
			LastRun: now.Add(1 * delay),
			Delay:   delay,
		},
		exp: 2 * delay,
	}}

	for _, c := range cases {
		got := c.job.computeFirstTimer(now)
		test.Assert(t, c.desc, c.exp, got)
	}
}
