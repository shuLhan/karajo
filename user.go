// SPDX-FileCopyrightText: 2023 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

package karajo

import (
	"fmt"
	"os"

	"github.com/shuLhan/share/lib/ini"
	"golang.org/x/crypto/bcrypt"
)

// User represent the account that can access Karajo user interface using
// name and password.
// The Password field store the bcrypt hash of plain password.
type User struct {
	Name     string
	Password string `ini:"::password"`
}

// loadUsers load user from file, return the map with user's name as key.
// If the file is not exist it will return empty users without an error.
func loadUsers(file string) (users map[string]*User, err error) {
	type container struct {
		Users map[string]*User `ini:"user"`
	}

	var (
		logp    = `loadUsers`
		content []byte
	)

	content, err = os.ReadFile(file)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}

	var cont = container{
		Users: make(map[string]*User),
	}

	err = ini.Unmarshal(content, &cont)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}

	users = cont.Users
	cont.Users = nil

	var (
		name string
		u    *User
	)
	for name, u = range users {
		u.Name = name
	}

	return users, nil
}

// authenticate return true if the hash of plain password match with user's
// Password.
func (u *User) authenticate(plain string) bool {
	var err = bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(plain))
	return err == nil
}
