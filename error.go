// SPDX-FileCopyrightText: 2023 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

package karajo

import (
	"net/http"

	liberrors "github.com/shuLhan/share/lib/errors"
)

var ErrJobPaused error = &liberrors.E{
	Code:    http.StatusPreconditionFailed,
	Name:    `ERR_JOB_PAUSED`,
	Message: `job is paused`,
}

var ErrJobMaxReached error = &liberrors.E{
	Code:    http.StatusTooManyRequests,
	Name:    `ERR_JOB_MAX_REACHED`,
	Message: `job has reached maximum running`,
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
