// SPDX-FileCopyrightText: 2023 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

package karajo

import libhttp "git.sr.ht/~shulhan/pakakeh.go/lib/http"

// ClientOptions define the options for Karajo HTTP client.
type ClientOptions struct {
	// Secret for API that require authorization, for example to pause or
	// resume a job.
	Secret string

	libhttp.ClientOptions
}
