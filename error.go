// SPDX-FileCopyrightText: 2023 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

package karajo

import (
	"net/http"

	liberrors "github.com/shuLhan/share/lib/errors"
)

// List of errors.
var (
	ErrJobAlreadyRun = liberrors.E{
		Code:    http.StatusTooManyRequests,
		Name:    `ERR_JOB_ALREADY_RUN`,
		Message: `job already run`,
	}
	ErrJobEmptyCommandsOrCall error = &liberrors.E{
		Code:    http.StatusBadRequest,
		Name:    `ERR_JOB_EMPTY_COMMANDS_OR_CALL`,
		Message: `empty commands or call handle`,
	}
	ErrJobForbidden error = &liberrors.E{
		Code:    http.StatusForbidden,
		Name:    `ERR_JOB_FORBIDDEN`,
		Message: `forbidden`,
	}
	ErrJobPaused error = &liberrors.E{
		Code:    http.StatusPreconditionFailed,
		Name:    `ERR_JOB_PAUSED`,
		Message: `job is paused`,
	}
)

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
