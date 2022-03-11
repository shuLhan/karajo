// SPDX-FileCopyrightText: 2021 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

package karajo

import "testing"

func TestLoadEnvironment(t *testing.T) {
	env, err := LoadEnvironment("testdata/karajo.conf")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("env: %+v\n", env)
	for x, job := range env.Jobs {
		t.Logf("env.Jobs[%d]: %+v\n", x, job)
	}
}
