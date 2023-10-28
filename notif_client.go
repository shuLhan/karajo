// SPDX-FileCopyrightText: 2023 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

package karajo

// notifClient generic client for sending notification.
type notifClient interface {
	// Send the job status and log.
	Send(jlog *JobLog)
}
