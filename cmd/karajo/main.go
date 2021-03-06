// Copyright 2021, Shulhan <ms@kilabit.info>. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"flag"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	"git.sr.ht/~shulhan/karajo"
	"github.com/shuLhan/share/lib/mlog"
)

func main() {
	mlog.SetPrefix("karajo:")

	var config string

	flag.StringVar(&config, "config", "", "the karajo configuration file")
	flag.Parse()

	if len(config) == 0 {
		flag.PrintDefaults()
		return
	}

	env, err := karajo.LoadEnvironment(config)
	if err != nil {
		mlog.Fatalf(err.Error())
	}

	k, err := karajo.New(env)
	if err != nil {
		mlog.Fatalf(err.Error())
	}

	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)
		<-c
		err := k.Stop()
		if err != nil {
			mlog.Errf(err.Error())
		}
	}()

	defer func() {
		err := recover()
		if err != nil {
			mlog.Errf("recover: %s\n", err)
			mlog.Flush()
			debug.PrintStack()
			os.Exit(1)
		}
	}()
	defer mlog.Flush()

	err = k.Start()
	if err != nil {
		mlog.Fatalf(err.Error())
	}
}
