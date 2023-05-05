// SPDX-FileCopyrightText: 2023 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

package karajo

import (
	"math/rand"
	"testing"

	"github.com/shuLhan/share/lib/test"
)

func TestSessionManager(t *testing.T) {
	rand.Seed(42)

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

	// Fill up the values to make it fail.

	sm.value[`FTiwaUT7G9GSS65BQQzOrW0BTgEPzZNA`] = expUser
	sm.value[`T3o4bnIGvsCe4lBrQJzFrXfxVhIrzHmc`] = expUser
	sm.value[`0FlaNsBlPcjJptPng4dCK6mPT1BTCjGJ`] = expUser
	sm.value[`BFrzrG0rzsl0eOI4G28wvNq9K3e1GW07`] = expUser
	sm.value[`ZwsldgKwLTudiA3O2FKgNadcJTHyJfAI`] = expUser

	key = sm.new(expUser)
	test.Assert(t, `sessionManager.new: failed`, ``, key)
}
