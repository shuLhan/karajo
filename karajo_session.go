// SPDX-FileCopyrightText: 2023 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

package karajo

import (
	"fmt"
	"net/http"
)

const (
	cookieName = `karajo`
)

// sessionNew generate and store new session for user.
func (k *Karajo) sessionNew(w http.ResponseWriter, user *User) (err error) {
	var (
		logp = `sessionNew`
		key  string
	)

	key = k.sm.new(user)
	if len(key) == 0 {
		return fmt.Errorf(`%s: failed to generate new session`, logp)
	}

	var cookie = &http.Cookie{
		Name:     cookieName,
		Value:    key,
		MaxAge:   86400, // One day in seconds.
		Path:     `/`,
		Secure:   false,
		HttpOnly: true,
	}

	http.SetCookie(w, cookie)

	return nil
}
