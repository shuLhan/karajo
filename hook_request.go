// SPDX-FileCopyrightText: 2023 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

package karajo

// HookRequest define the base request for triggering Hook using HTTP POST
// with JSON body.
type HookRequest struct {
	Epoch int64 `json:"_karajo_epoch"`
}
