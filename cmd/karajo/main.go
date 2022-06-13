// SPDX-FileCopyrightText: 2021 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"flag"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	"github.com/shuLhan/share/lib/memfs"
	"github.com/shuLhan/share/lib/mlog"

	"git.sr.ht/~shulhan/karajo"
)

const (
	cmdEmbed = "embed"
)

func main() {
	mlog.SetPrefix("karajo:")
	defer mlog.Flush()

	var (
		env    *karajo.Environment
		k      *karajo.Karajo
		mfs    *memfs.MemFS
		config string
		cmd    string
		err    error
	)

	flag.StringVar(&config, "config", "", "the karajo configuration file")
	flag.Parse()

	cmd = flag.Arg(0)

	switch cmd {
	case cmdEmbed:
		var opts = memfs.Options{
			Root: "_www",
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

	k, err = karajo.New(env)
	if err != nil {
		mlog.Fatalf(err.Error())
	}

	go func() {
		var (
			c   chan os.Signal = make(chan os.Signal, 1)
			err error
		)

		signal.Notify(c, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)
		<-c
		err = k.Stop()
		if err != nil {
			mlog.Errf(err.Error())
		}
	}()

	defer func() {
		var panicMsg = recover()
		if panicMsg != nil {
			mlog.Errf("recover: %s\n", panicMsg)
			mlog.Flush()
			debug.PrintStack()
			os.Exit(1)
		}
	}()

	err = k.Start()
	if err != nil {
		mlog.Fatalf(err.Error())
	}
}
