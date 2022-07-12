// SPDX-FileCopyrightText: 2021 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"flag"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/shuLhan/share/lib/memfs"
	"github.com/shuLhan/share/lib/mlog"

	"git.sr.ht/~shulhan/ciigo"
	"git.sr.ht/~shulhan/karajo"
)

const (
	cmdEmbed = "embed"
)

func main() {
	mlog.SetPrefix("karajo:")
	defer mlog.Flush()

	var (
		env     *karajo.Environment
		k       *karajo.Karajo
		mfs     *memfs.MemFS
		running chan bool
		config  string
		cmd     string
		err     error
		isDev   bool
	)

	flag.StringVar(&config, "config", "", "the karajo configuration file")
	flag.BoolVar(&isDev, "dev", false, "enable development mode")
	flag.Parse()

	cmd = flag.Arg(0)

	switch cmd {
	case cmdEmbed:
		var opts = memfs.Options{
			Root: "_www",
			Excludes: []string{
				`.*\.adoc$`,
			},
			Embed: memfs.EmbedOptions{
				CommentHeader: `// SPDX-FileCopyrightText: 2021 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later
`,
				PackageName: "karajo",
				VarName:     "memfsWww",
				GoFileName:  "memfs_www.go",
			},
		}

		mfs, err = memfs.New(&opts)
		if err != nil {
			mlog.Fatalf(err.Error())
		}

		err = mfs.GoEmbed()
		if err != nil {
			mlog.Fatalf(err.Error())
		}
		return
	}

	if len(config) == 0 {
		flag.PrintDefaults()
		return
	}

	env, err = karajo.LoadEnvironment(config)
	if err != nil {
		mlog.Fatalf(err.Error())
	}

	env.IsDevelopment = isDev

	k, err = karajo.New(env)
	if err != nil {
		mlog.Fatalf(err.Error())
	}

	defer func() {
		var panicMsg = recover()
		if panicMsg != nil {
			mlog.Errf("recover: %s", panicMsg)
			mlog.Flush()
			debug.PrintStack()
			os.Exit(1)
		}
	}()

	if env.IsDevelopment {
		running = make(chan bool, 1)
		go watchWww(running)
		go watchWwwDoc()
	}

	go func() {
		var (
			c   chan os.Signal = make(chan os.Signal, 1)
			err error
		)

		signal.Notify(c, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)
		<-c
		if env.IsDevelopment {
			running <- false
			<-running
		}
		err = k.Stop()
		if err != nil {
			mlog.Errf(err.Error())
		}
	}()

	err = k.Start()
	if err != nil {
		mlog.Fatalf(err.Error())
	}
}

// watchWww run the development mode to watch changes in .adoc files inside
// _www/karajo/doc, convert, and embed them.
func watchWww(running chan bool) {
	var (
		tick    = time.NewTicker(3 * time.Second)
		mfsOpts = memfs.Options{
			Root: "_www",
			Excludes: []string{
				`.*\.adoc$`,
			},
			Embed: memfs.EmbedOptions{
				CommentHeader: `// SPDX-FileCopyrightText: 2021 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later
`,
				PackageName: "karajo",
				VarName:     "memfsWww",
				GoFileName:  "memfs_www.go",
			},
		}
		isRunning = true

		mfsWww   *memfs.MemFS
		dw       *memfs.DirWatcher
		nChanges int
		err      error
	)

	mfsWww, err = memfs.New(&mfsOpts)
	if err != nil {
		mlog.Fatalf(err.Error())
	}

	dw, err = mfsWww.Watch(memfs.WatchOptions{})
	if err != nil {
		mlog.Fatalf(err.Error())
	}

	for isRunning {
		select {
		case <-dw.C:
			nChanges++

		case <-tick.C:
			if nChanges == 0 {
				continue
			}

			mlog.Outf("--- %d changes", nChanges)
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
		mlog.Outf("--- %d changes", nChanges)
		err = mfsWww.GoEmbed()
		if err != nil {
			mlog.Errf(err.Error())
		}
	}
	dw.Stop()
	running <- false
}

func watchWwwDoc() {
	var (
		logp        = "watchWwwDoc"
		convertOpts = ciigo.ConvertOptions{
			Root: "_www/karajo/doc",
		}

		err error
	)

	err = ciigo.Watch(&convertOpts)
	if err != nil {
		mlog.Fatalf("%s: %s", logp, err)
	}
}
