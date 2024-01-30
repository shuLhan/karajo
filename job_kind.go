// SPDX-FileCopyrightText: 2023 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

package karajo

// jobKind define the type of Job.
type jobKind string

// List of job kind.
const (
	jobKindExec jobKind = `job`
	jobKindHTTP jobKind = `job_http`
)
