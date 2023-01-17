// SPDX-FileCopyrightText: 2023 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

package karajo

import (
	"net/http"

	liberrors "github.com/shuLhan/share/lib/errors"
)

// List of errors.
var (
	ErrJobEmptyCommandsOrCall error = &liberrors.E{
		Code:    http.StatusBadRequest,
		Name:    `ERR_JOB_EMPTY_COMMANDS_OR_CALL`,
		Message: "empty commands or call handle",
	}
	ErrJobForbidden error = &liberrors.E{
		Code:    http.StatusForbidden,
		Name:    `ERR_JOB_FORBIDDEN`,
		Message: "forbidden",
	}
	ErrJobInvalidSecret error = &liberrors.E{
		Code:    http.StatusBadRequest,
		Name:    `ERR_JOB_INVALID_SECRET`,
		Message: "invalid or empty secret",
	}
	ErrJobMaxReached error = &liberrors.E{
		Code:    http.StatusTooManyRequests,
		Name:    `ERR_JOB_MAX_REACHED`,
		Message: `job has reached maximum running`,
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
