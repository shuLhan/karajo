// Copyright 2021, Shulhan <ms@kilabit.info>. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

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
