// SPDX-FileCopyrightText: 2021 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

/*
Program karajo implements HTTP workers and manager, similar to cron but
works and manageable with HTTP.

Karajo has the web user interface (WUI) for monitoring the jobs that can
be accessed at "http://127.0.0.1:31937/karajo" .

A single instance of karajo can be configured through code or from file
using INI file format.

Features,

  - Running job on specific interval.
  - Running job on specific schedule.
  - Trigger HTTP request to external server on specific interval.
  - Preserve the job states on restart.
  - Able to pause and resume specific job.
  - Trigger job using HTTP request (webhook). Supported webhook are `github`,
    `sourcehut`, or custom `hmac-sha256` (default).
  - HTTP APIs to programmatically interact with server
  - User authentication

Workflow on karajo,

	                karajo
	              /-----------------------------\
	              |                             |
	              |   +---+         +---+       |
	              |   |   | timer   |   | timer |
	              |   |   v         |   v       |
	              | +---------+   +-------+     |
	INTERNET <----- | JobHttp |   | Job   | <----- INTERNET
	              | +---------+   +-------+     |
	              \------------------|----------/
	                                 |
	                                 v
	                      +-----------------+
	                      | Commands / Call |
	                      +-----------------+`
*/
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

	var (
		env    *karajo.Env
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

	env, err = karajo.LoadEnv(config)
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
			c chan os.Signal = make(chan os.Signal, 1)
		)

		signal.Notify(c, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)
		<-c
		var err2 = k.Stop()
		if err2 != nil {
			mlog.Errf(err2.Error())
		}
	}()

	err = k.Start()
	if err != nil {
		mlog.Fatalf(err.Error())
	}

	mlog.Flush()
}
