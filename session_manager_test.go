// SPDX-FileCopyrightText: 2023 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

package karajo

import (
	"testing"

	"git.sr.ht/~shulhan/pakakeh.go/lib/test"
)

func TestSessionManager(t *testing.T) {
	var (
		sm      = newSessionManager()
		expUser = &User{
			Name: `test`,
		}

		gotUser *User
		key     string
	)

	key = sm.new(expUser)
	gotUser = sm.get(key)

	test.Assert(t, `sessionManager.get: exist`, expUser, gotUser)

	sm.delete(key)

	var nilUser *User
	gotUser = sm.get(key)
	test.Assert(t, `sessionManager.get: not exist`, nilUser, gotUser)

	// Test generate new key.

	key = sm.new(expUser)

	test.Assert(t, `sessionManager.new:`, 32, len(key))
}
