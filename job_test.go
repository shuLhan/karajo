// SPDX-FileCopyrightText: 2023 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

package karajo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	libhttp "github.com/shuLhan/share/lib/http"
	"github.com/shuLhan/share/lib/test"
)

// TestJob_handleHttp test Job's Call with HTTP request.
func TestJob_handleHttp(t *testing.T) {
	var (
		testBaseDir = t.TempDir()
		env         = Environment{
			DirBase: testBaseDir,
			Secret:  `s3cret`,
		}
		job = Job{
			JobBase: JobBase{
				Name: `Test job handle HTTP`,
			},
			Path:   `/test-job-handle-http`,
			Secret: `s3cret`,
			Call: func(hlog io.Writer, _ *libhttp.EndpointRequest) error {
				fmt.Fprintf(hlog, `Output from Call`)
				return nil
			},
		}

		tdata *test.Data
		err   error
	)

	tdata, err = test.LoadData(`testdata/job_handleHttp_test.txt`)
	if err != nil {
		t.Fatal(err)
	}

	err = env.init()
	if err != nil {
		t.Fatal(err)
	}

	err = job.init(&env, job.Name)
	if err != nil {
		t.Fatal(err)
	}

	var (
		jobReq = JobHttpRequest{
			Epoch: testTimeNow.Unix(),
		}
		epr = libhttp.EndpointRequest{
			HttpRequest: &http.Request{
				Header: http.Header{},
			},
		}
		sign string
	)

	epr.RequestBody, err = json.Marshal(&jobReq)
	if err != nil {
		t.Fatal(err)
	}

	sign = Sign(epr.RequestBody, []byte(job.Secret))
	epr.HttpRequest.Header.Set(HeaderNameXKarajoSign, sign)

	var (
		buf bytes.Buffer
		got []byte
		exp []byte
	)

	got, err = job.handleHttp(&epr)
	if err != nil {
		t.Fatal(err)
	}

	err = json.Indent(&buf, got, ``, `  `)
	if err != nil {
		t.Fatal(err)
	}

	got = buf.Bytes()
	exp = tdata.Output[`handleHttp_response.json`]
	test.Assert(t, `handleHttp_response`, string(exp), string(got))

	<-job.finished

	job.Lock()
	got, err = json.MarshalIndent(&job, ``, `  `)
	if err != nil {
		job.Unlock()
		t.Fatal(err)
	}
	job.Unlock()

	exp = tdata.Output[`job_after.json`]
	test.Assert(t, `TestJob_Call`, string(exp), string(got))
}

// TestJob_Start test Job's Call with timer.
func TestJob_Start(t *testing.T) {
	var (
		testBaseDir = t.TempDir()
		env         = Environment{
			DirBase: testBaseDir,
			Secret:  `s3cret`,
		}
		job = Job{
			JobBase: JobBase{
				Name:     `Test job timer`,
				Interval: time.Minute,
			},
			Path:   `/test-job-timer`,
			Secret: `s3cret`,
			Call: func(hlog io.Writer, _ *libhttp.EndpointRequest) error {
				fmt.Fprintf(hlog, `Output from Call`)
				return nil
			},
		}

		tdata *test.Data
		got   []byte
		exp   []byte
		err   error
	)

	tdata, err = test.LoadData(`testdata/job_Start_test.txt`)
	if err != nil {
		t.Fatal(err)
	}

	err = env.init()
	if err != nil {
		t.Fatal(err)
	}

	err = job.init(&env, job.Name)
	if err != nil {
		t.Fatal(err)
	}

	go job.Start()
	defer job.Stop()

	<-job.finished

	job.Lock()
	got, err = json.MarshalIndent(&job, ``, `  `)
	if err != nil {
		job.Unlock()
		t.Fatal(err)
	}
	job.Unlock()

	exp = tdata.Output[`job_after.json`]
	test.Assert(t, `TestJob_Call`, string(exp), string(got))
}
