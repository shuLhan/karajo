// SPDX-FileCopyrightText: 2023 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

package karajo

import (
	"net/http"

	liberrors "git.sr.ht/~shulhan/pakakeh.go/lib/errors"
)

// errAuthLogin error for failed authentication due to invalid user or
// password.
var errAuthLogin = liberrors.E{
	Code:    http.StatusBadRequest,
	Name:    `ERR_AUTH_LOGIN`,
	Message: `invalid user name and/or password`,
}

var errJobAlreadyRun = liberrors.E{
	Code:    http.StatusTooManyRequests,
	Name:    `ERR_JOB_ALREADY_RUN`,
	Message: `job already run`,
}

var errJobCanceled = liberrors.E{
	Code:    http.StatusGone,
	Name:    `ERR_JOB_CANCELED`,
	Message: `job is canceled`,
}

var errJobEmptyCommandsOrCall = liberrors.E{
	Code:    http.StatusBadRequest,
	Name:    `ERR_JOB_EMPTY_COMMANDS_OR_CALL`,
	Message: `empty commands or call handle`,
}

var errJobForbidden = liberrors.E{
	Code:    http.StatusForbidden,
	Name:    `ERR_JOB_FORBIDDEN`,
	Message: `forbidden`,
}

var errJobPaused = liberrors.E{
	Code:    http.StatusPreconditionFailed,
	Name:    `ERR_JOB_PAUSED`,
	Message: `job is paused`,
}

func errInvalidJobID(id string) error {
	return &liberrors.E{
		Code:    http.StatusBadRequest,
		Name:    `ERR_INVALID_JOB_ID`,
		Message: `invalid or empty job id: ` + id,
	}
}

func errJobNotFound(jobPath string) error {
	return &liberrors.E{
		Code:    http.StatusNotFound,
		Name:    `ERR_JOB_NOT_FOUND`,
		Message: `job not found: ` + jobPath,
	}
}
