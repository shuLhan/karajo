// SPDX-FileCopyrightText: 2023 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

package karajo

import (
	"testing"

	"github.com/shuLhan/share/lib/test"
)

func TestLoadUsers(t *testing.T) {
	var (
		expUsers = map[string]*User{
			`test`: &User{
				Name:     `test`,
				Password: `$2a$10$9XMRfqpnzY2421fwYm5dd.CidJf7dHHWIESeeNGXuajHRf.Lqzy7a`,
			},
		}
		gotUsers map[string]*User
		err      error
	)

	gotUsers, err = loadUsers(`testdata/etc/karajo/user.conf`)
	if err != nil {
		t.Fatal(err)
	}

	test.Assert(t, `loadUsers`, expUsers, gotUsers)
}

func TestUser_authenticate(t *testing.T) {
	var (
		u = &User{
			Name:     `test`,
			Password: `$2a$10$9XMRfqpnzY2421fwYm5dd.CidJf7dHHWIESeeNGXuajHRf.Lqzy7a`,
		}
		got bool
	)

	got = u.authenticate(`s3cr3t`)
	test.Assert(t, `authenticate: invalid`, false, got)

	got = u.authenticate(`s3cret`)
	test.Assert(t, `authenticate: valid`, true, got)
}
