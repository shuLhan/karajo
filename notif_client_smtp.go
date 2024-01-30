// SPDX-FileCopyrightText: 2023 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

package karajo

import (
	"fmt"

	"github.com/shuLhan/share/lib/email"
	"github.com/shuLhan/share/lib/mlog"
	"github.com/shuLhan/share/lib/smtp"
)

// clientSMTP client for SMTP.
type clientSMTP struct {
	conn *smtp.Client
	opts smtp.ClientOptions
	env  EnvNotif
}

// newClientSMTP create new client for SMTP.
func newClientSMTP(envNotif EnvNotif) (cl *clientSMTP, err error) {
	var logp = `newClientSMTP`

	cl = &clientSMTP{
		env: envNotif,
		opts: smtp.ClientOptions{
			ServerUrl:     envNotif.SMTPServer,
			AuthUser:      envNotif.SMTPUser,
			AuthPass:      envNotif.SMTPPassword,
			AuthMechanism: smtp.SaslMechanismPlain,
			Insecure:      envNotif.SMTPInsecure,
		},
	}

	// Test connecting and authenticated with the server.
	cl.conn, err = smtp.NewClient(cl.opts)
	if err != nil {
		return nil, fmt.Errorf(`%s: %s: %w`, logp, cl.opts.ServerUrl, err)
	}

	_, _ = cl.conn.Quit()

	return cl, nil
}

// Send the job status and log to user.
func (cl *clientSMTP) Send(jlog *JobLog) {
	var (
		logp = `clientSMTP.Send`
		msg  = email.Message{}

		v    string
		data []byte
		err  error
	)

	err = msg.SetFrom(cl.env.From)
	if err != nil {
		mlog.Errf(`%s: %s`, logp, err)
		return
	}
	for _, v = range cl.env.To {
		err = msg.AddTo(v)
		if err != nil {
			mlog.Errf(`%s: To %s: %s`, logp, v, err)
			return
		}
	}

	v = fmt.Sprintf(`%s: %s: #%d: %s`, jlog.jobKind, jlog.JobID, jlog.Counter, jlog.Status)
	msg.SetSubject(v)

	err = msg.SetBodyText(jlog.content)
	if err != nil {
		mlog.Errf(`%s: %s`, logp, err)
		return
	}

	data, err = msg.Pack()
	if err != nil {
		mlog.Errf(`%s: %s`, logp, err)
		return
	}

	var mailtx = smtp.NewMailTx(cl.env.From, cl.env.To, data)

	cl.conn, err = smtp.NewClient(cl.opts)
	if err != nil {
		mlog.Errf(`%s: %s: %s`, logp, cl.opts.ServerUrl, err)
		return
	}

	_, err = cl.conn.MailTx(mailtx)
	if err != nil {
		mlog.Errf(`%s: %s`, logp, err)
		return
	}

	_, err = cl.conn.Quit()
	if err != nil {
		mlog.Errf(`%s: %s`, logp, err)
		return
	}
}
