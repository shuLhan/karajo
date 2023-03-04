// SPDX-FileCopyrightText: 2021 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"syscall"

	"github.com/shuLhan/share/lib/mlog"

	"git.sr.ht/~shulhan/karajo"
)

const (
	cmdVersion = `version`
)

func main() {
	mlog.SetPrefix(`karajo:`)
	defer mlog.Flush()

	var (
		env    *karajo.Environment
		k      *karajo.Karajo
		config string
		cmd    string
		err    error
	)

	flag.StringVar(&config, `config`, ``, `the karajo configuration file`)
	flag.Parse()

	cmd = flag.Arg(0)
	cmd = strings.ToLower(cmd)

	switch cmd {
	case cmdVersion:
		fmt.Println(`karajo version ` + karajo.Version)
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

	defer func() {
		var panicMsg = recover()
		if panicMsg != nil {
			mlog.Errf(`recover: %s`, panicMsg)
			mlog.Flush()
			debug.PrintStack()
			os.Exit(1)
		}
	}()

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

	err = k.Start()
	if err != nil {
		mlog.Fatalf(err.Error())
	}
}
