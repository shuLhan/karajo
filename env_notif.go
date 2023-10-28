// SPDX-FileCopyrightText: 2023 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

package karajo

import (
	"fmt"
	"log"
	"os"
)

const (
	notifKindEmail = `email`
)

// EnvNotif environment for notification.
type EnvNotif struct {
	Name         string
	Kind         string   `ini:"::kind"`
	SmtpServer   string   `ini:"::smtp_server"`
	SmtpUser     string   `ini:"::smtp_user"`
	SmtpPassword string   `ini:"::smtp_password"`
	From         string   `ini:"::from"`
	To           []string `ini:"::to"`
	SmtpInsecure bool     `ini:"::smtp_insecure"`
}

// init initialize the envNotif.
func (envNotif *EnvNotif) init() {
	if envNotif.SmtpUser[0] == '$' {
		envNotif.SmtpUser = os.Getenv(envNotif.SmtpUser)
	}
	if envNotif.SmtpPassword[0] == '$' {
		envNotif.SmtpPassword = os.Getenv(envNotif.SmtpPassword)
	}
}

// createClient create client for notification based on its kind.
// It will return an error if kind is unknown or the client failed to created.
func (envNotif *EnvNotif) createClient() (cl notifClient, err error) {
	var logp = `createClient`

	switch envNotif.Kind {
	case notifKindEmail:
		cl, err = newClientSmtp(*envNotif)
	default:
		err = fmt.Errorf(`unknown kind %q`, envNotif.Kind)
	}
	if err != nil {
		return nil, fmt.Errorf(`%s: %s: %w`, logp, envNotif.Name, err)
	}

	log.Printf(`notif %q: connected`, envNotif.Name)

	return cl, nil
}
