// Copyright 2021, Shulhan <ms@kilabit.info>. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package karajo

//
// LogOptions define the log directory and optional log file suffix for each
// job.
//
type LogOptions struct {
	Dir            string `ini:"::dir"`
	FilenamePrefix string `ini:"::filename_prefix"`
}
