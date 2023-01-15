// SPDX-FileCopyrightText: 2023 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

package karajo

// JobHttpRequest define the base request for triggering Job using HTTP POST
// with JSON body.
type JobHttpRequest struct {
	Epoch int64 `json:"_karajo_epoch"`
}
