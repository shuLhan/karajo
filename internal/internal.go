// SPDX-FileCopyrightText: 2023 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

// Package internal provides function for development, including converting
// ".adoc" files to HTML, embedding assets into Go code, and watching files
// for previewing documentation.
package internal

import (
	"fmt"

	"git.sr.ht/~shulhan/ciigo"
	"git.sr.ht/~shulhan/pakakeh.go/lib/memfs"
	"git.sr.ht/~shulhan/pakakeh.go/lib/mlog"
)

// ConvertAdocToHTML convert adoc files to HTML files.
func ConvertAdocToHTML() (err error) {
	var (
		logp        = `ConvertAdocToHTML`
		convertOpts = ciigo.ConvertOptions{
			Root: `_www/karajo/doc`,
		}
	)

	err = ciigo.Convert(convertOpts)
	if err != nil {
		return fmt.Errorf(`%s: %w`, logp, err)
	}
	return nil
}

// GenerateMemfs generate the memfs instance to start watching or embedding
// the _www directory.
func GenerateMemfs() (mfs *memfs.MemFS, err error) {
	var (
		opts = memfs.Options{
			Root: `_www`,
			Excludes: []string{
				`.*\.adoc$`,
			},
			Embed: memfs.EmbedOptions{
				CommentHeader: `// SPDX-FileCopyrightText: 2021 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later
`,
				PackageName: `karajo`,
				VarName:     `memfsWww`,
				GoFileName:  `memfs_www.go`,
			},
		}
	)

	mfs, err = memfs.New(&opts)
	if err != nil {
		return nil, err
	}

	return mfs, nil
}

// WatchWww watch file changes inside _www directory and then embed them into
// Go code.
func WatchWww(running chan bool) {
	var (
		mfsWww  *memfs.MemFS
		changes <-chan []*memfs.Node
		err     error
	)

	mfsWww, err = GenerateMemfs()
	if err != nil {
		mlog.Fatalf(err.Error())
	}

	changes, err = mfsWww.Watch(memfs.WatchOptions{})
	if err != nil {
		mlog.Fatalf(err.Error())
	}

	// Embed first ...
	err = mfsWww.GoEmbed()
	if err != nil {
		mlog.Fatalf(err.Error())
	}

	var isRunning = true
	for isRunning {
		select {
		case <-changes:
			err = mfsWww.GoEmbed()
			if err != nil {
				mlog.Errf(err.Error())
			}

		case <-running:
			isRunning = false
		}
	}

	// Run GoEmbed for the last time.
	err = mfsWww.GoEmbed()
	if err != nil {
		mlog.Errf(err.Error())
	}
	running <- false
}

// WatchWwwDoc watch for .adoc file changes inside _www/karajo/doc directory
// and then convert them to HTML.
func WatchWwwDoc() {
	var (
		logp        = `watchWwwDoc`
		convertOpts = ciigo.ConvertOptions{
			Root: `_www/karajo/doc`,
		}

		err error
	)

	err = ciigo.Watch(convertOpts)
	if err != nil {
		mlog.Fatalf(`%s: %s`, logp, err)
	}
}
