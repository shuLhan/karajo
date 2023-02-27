// SPDX-FileCopyrightText: 2023 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

package internal

import (
	"time"

	"git.sr.ht/~shulhan/ciigo"
	"github.com/shuLhan/share/lib/memfs"
	"github.com/shuLhan/share/lib/mlog"

	"git.sr.ht/~shulhan/karajo"
)

// WatchWww watch file changes inside _www directory and then embed them into
// Go code.
func WatchWww(running chan bool) {
	var (
		tick = time.NewTicker(3 * time.Second)

		mfsWww *memfs.MemFS
		dw     *memfs.DirWatcher
		err    error
	)

	mfsWww, err = karajo.GenerateMemfs()
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
