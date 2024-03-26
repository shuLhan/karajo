// SPDX-FileCopyrightText: 2023 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

package karajo

import "git.sr.ht/~shulhan/pakakeh.go/lib/ascii"

// sessionManager manage the active session that map to authenticated user.
type sessionManager struct {
	value map[string]*User
}

// newSessionManager create new session manager.
func newSessionManager() (sm *sessionManager) {
	sm = &sessionManager{
		value: make(map[string]*User),
	}
	return sm
}

// new create new session for user u.
func (sm *sessionManager) new(u *User) (key string) {
	var (
		sessb []byte
		n     int
		ok    bool
	)
	for n < 5 {
		sessb = ascii.Random([]byte(ascii.LettersNumber), 32)
		key = string(sessb)
		_, ok = sm.value[key]
		if !ok {
			sm.value[key] = u
			return key
		}
		n++
	}
	// Failed to generate unique session, return empty key.
	return ``
}

// get the user related to session key.
// It will return nil if user is not exist.
func (sm *sessionManager) get(key string) (u *User) {
	u = sm.value[key]
	return u
}

// delete the session from storage.
func (sm *sessionManager) delete(key string) {
	delete(sm.value, key)
}
