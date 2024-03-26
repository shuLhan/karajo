// SPDX-FileCopyrightText: 2023 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

// Program karajo-example example creating Karajo jobs by code.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	libhttp "git.sr.ht/~shulhan/pakakeh.go/lib/http"
	"git.sr.ht/~shulhan/pakakeh.go/lib/mlog"

	"git.sr.ht/~shulhan/karajo"
	"git.sr.ht/~shulhan/karajo/internal"
)

func main() {
	var (
		env *karajo.Env
		k   *karajo.Karajo
		err error
	)

	env, err = karajo.LoadEnv(`internal/cmd/karajo-example/testdata/etc/karajo/karajo.conf`)
	if err != nil {
		log.Fatal(err)
	}

	env.ExecJobs = make(map[string]*karajo.JobExec)

	env.ExecJobs[`interval-1m-code`] = &karajo.JobExec{
		JobBase: karajo.JobBase{
			Description: `JobExec with interval 1 minute, initialize by code.`,
			Interval:    1 * time.Minute,
		},
		Path: `/interval-1m-code`,
		Call: helloWorldFromInterval,
	}

	env.ExecJobs[`schedule-hourly-5m-code`] = &karajo.JobExec{
		JobBase: karajo.JobBase{
			Description: `JobExec with schedule every hour at minutes 5.`,
			Schedule:    `hourly@0,5,10,15,20,25,30,35,40,45,50,55`,
		},
		Path: `/schedule-hourly-5m-code-code`,
		Call: helloWorldFromSchedule,
	}

	env.ExecJobs[`call-sleep-30m`] = &karajo.JobExec{
		JobBase: karajo.JobBase{
			Description: `Job that run sleep for 30 minutes.`,
		},
		Path: `/call-sleep-30m`,
		Call: callSleep30m,
	}

	env.ExecJobs[`webhook-github-code`] = &karajo.JobExec{
		JobBase: karajo.JobBase{
			Description: `Webhook using github authentication.`,
		},
		AuthKind: `github`,
		Secret:   `s3cret`,
		Path:     `/webhook-github-code`,
		Call:     webhookWithGithub,
	}

	// Example of JobHTTP.

	env.HTTPJobs = make(map[string]*karajo.JobHTTP)
	env.HTTPJobs[`interval-90s-code`] = &karajo.JobHTTP{
		JobBase: karajo.JobBase{
			Description: `Trigger our webhook-github every 90 seconds by code.`,
			Interval:    90 * time.Second,
		},
		Secret:          `s3cret`,
		HeaderSign:      `X-Hub-Signature-256`,
		HTTPMethod:      `POST`,
		HTTPURL:         `/karajo/api/job_exec/run/webhook-github`,
		HTTPRequestType: `json`,
	}

	env.HTTPJobs[`schedule-6m-code`] = &karajo.JobHTTP{
		JobBase: karajo.JobBase{
			Description: `Trigger our webhook-github-code by schedule every 6m.`,
			Schedule:    `hourly@0,6,12,18,24,30,36,42,48,54`,
		},
		Secret:          `s3cret`,
		HeaderSign:      `X-Hub-Signature-256`,
		HTTPMethod:      `POST`,
		HTTPURL:         `/karajo/api/job_exec/run/webhook-github-code`,
		HTTPRequestType: `json`,
	}

	k, err = karajo.New(env)
	if err != nil {
		log.Fatal(err)
	}

	go watcher(k)

	err = k.Start()
	if err != nil {
		log.Fatal(err)
	}
}

func helloWorldFromInterval(_ context.Context, log io.Writer, _ *libhttp.EndpointRequest) error {
	fmt.Fprintln(log, `Hello world from interval with code`)
	return nil
}

func helloWorldFromSchedule(_ context.Context, log io.Writer, _ *libhttp.EndpointRequest) error {
	fmt.Fprintln(log, `Hello world from schedule`)
	return nil
}

func webhookWithGithub(_ context.Context, log io.Writer, _ *libhttp.EndpointRequest) error {
	fmt.Fprintln(log, `Hello world from Webhook github`)
	return nil
}

func callSleep30m(ctx context.Context, log io.Writer, _ *libhttp.EndpointRequest) error {
	fmt.Fprintln(log, `Sleeping for 30 minutes...`)
	var timer = time.NewTimer(30 * time.Minute)
	select {
	case <-timer.C:
		fmt.Fprintln(log, `Waking up...`)
	case <-ctx.Done():
		return errors.New(`job cancelled by context`)
	}
	return nil
}

func watcher(k *karajo.Karajo) {
	var running = make(chan bool, 1)

	go internal.WatchWww(running)
	go internal.WatchWwwDoc()

	var (
		c   = make(chan os.Signal, 1)
		err error
	)

	signal.Notify(c, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)
	<-c
	running <- false
	<-running
	err = k.Stop()
	if err != nil {
		mlog.Errf(err.Error())
	}
}
