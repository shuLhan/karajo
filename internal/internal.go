// SPDX-FileCopyrightText: 2023 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

package internal

import (
	"fmt"
	"time"

	"git.sr.ht/~shulhan/ciigo"
	"github.com/shuLhan/share/lib/memfs"
	"github.com/shuLhan/share/lib/mlog"
)

// ConvertAdocToHtml convert adoc files to HTML files.
func ConvertAdocToHtml() (err error) {
	var (
		logp        = `ConvertAdocToHtml`
		convertOpts = ciigo.ConvertOptions{
			Root: `_www/karajo/doc`,
		}
	)

	err = ciigo.Convert(&convertOpts)
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
		tick = time.NewTicker(3 * time.Second)

		mfsWww *memfs.MemFS
		dw     *memfs.DirWatcher
		err    error
	)

	mfsWww, err = GenerateMemfs()
	if err != nil {
		mlog.Fatalf(err.Error())
	}

	dw, err = mfsWww.Watch(memfs.WatchOptions{})
	if err != nil {
		mlog.Fatalf(err.Error())
	}

	// Embed first ...
	err = mfsWww.GoEmbed()
	if err != nil {
		mlog.Fatalf(err.Error())
	}

	var (
		isRunning = true
		nChanges  int
	)

	for isRunning {
		select {
		case <-dw.C:
			nChanges++

		case <-tick.C:
			if nChanges == 0 {
				continue
			}

			mlog.Outf(`--- %d changes`, nChanges)
			err = mfsWww.GoEmbed()
			if err != nil {
				mlog.Errf(err.Error())
			}
			nChanges = 0

		case <-running:
			isRunning = false
		}
	}

	// Run GoEmbed for the last time.
	if nChanges > 0 {
		mlog.Outf(`--- %d changes`, nChanges)
		err = mfsWww.GoEmbed()
		if err != nil {
			mlog.Errf(err.Error())
		}
	}
	dw.Stop()
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

	err = ciigo.Watch(&convertOpts)
	if err != nil {
		mlog.Fatalf(`%s: %s`, logp, err)
	}
}
