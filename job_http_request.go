// SPDX-FileCopyrightText: 2023 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

package karajo

// JobHTTPRequest define the base request for managing Job or JobHTTP using
// HTTP POST with JSON body.
type JobHTTPRequest struct {
	ID    string `json:"id" form:"id"`
	Epoch int64  `json:"_karajo_epoch" form:"_karajo_epoch"`
}
