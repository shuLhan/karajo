// SPDX-FileCopyrightText: 2023 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

package karajo

import (
	"net/http"

	liberrors "github.com/shuLhan/share/lib/errors"
)

func errInvalidJobID(id string) error {
	return &liberrors.E{
		Code:    http.StatusBadRequest,
		Name:    `ERR_INVALID_JOB_ID`,
		Message: `invalid or empty job id: ` + id,
	}
}

func errHookNotFound(hookPath string) error {
	return &liberrors.E{
		Code:    http.StatusNotFound,
		Name:    `ERR_HOOK_NOT_FOUND`,
		Message: `hook not found: ` + hookPath,
	}
}
