// SPDX-FileCopyrightText: 2023 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

package karajo

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	libhttp "github.com/shuLhan/share/lib/http"
	"github.com/shuLhan/share/lib/mlog"
	"github.com/shuLhan/share/lib/test"
)

var (
	testTimeNow = time.Date(2023, time.January, 9, 0, 0, 0, 0, time.UTC)

	testEnv    *Environment
	testClient *Client
)

func TestMain(m *testing.M) {
	mlog.SetPrefix(``)
	mlog.SetTimeFormat(``)

	TimeNow = func() time.Time {
		return testTimeNow
	}

	os.Exit(m.Run())
}

func TestKarajo_apis(t *testing.T) {
	var (
		tdata   *test.Data
		httpJob *JobHttp
		karajo  *Karajo
		err     error
	)

	tdata, err = test.LoadData(`testdata/api_test.txt`)
	if err != nil {
		t.Fatal(err)
	}

	testEnv, err = ParseEnvironment(tdata.Input[`test.conf`])
	if err != nil {
		t.Fatal(err)
	}

	// Overwrite the base directory to make the output predictable.
	testEnv.DirBase = t.TempDir()

	karajo, err = New(testEnv)
	if err != nil {
		t.Fatal(err)
	}

	// Set the job LastRun to the current time so it will not run when
	// server started.
	for _, httpJob = range testEnv.HttpJobs {
		httpJob.LastRun = testTimeNow
	}

	go func() {
		var err = karajo.Start()
		if err != nil {
			log.Fatal(err)
		}
	}()

	defer func() {
		var err = karajo.Stop()
		if err != nil {
			log.Fatal(err)
		}
	}()

	var clientOpts = ClientOptions{
		ClientOptions: libhttp.ClientOptions{
			ServerUrl: fmt.Sprintf(`http://%s`, testEnv.ListenAddress),
		},
		Secret: `s3cret`,
	}
	testClient = NewClient(clientOpts)
	waitServerAlive(t, testClient)

	t.Run(`apiEnvironment`, func(tt *testing.T) {
		testKarajo_apiEnvironment(tt, tdata, testClient)
	})

	t.Run(`apiJobRun_success`, func(tt *testing.T) {
		testKarajo_apiJobRun_success(tt, tdata, testClient)
	})
	t.Run(`apiJobRun_notfound`, func(tt *testing.T) {
		testKarajo_apiJobRun_notfound(tt, tdata, testClient)
	})
	t.Run(`apiJobLog`, func(tt *testing.T) {
		testKarajo_apiJobLog(tt, tdata, testClient)
	})

	t.Run(`apiJobHtpp_success`, func(tt *testing.T) {
		testKarajo_apiJobHttp_success(tt, tdata, testClient)
	})
	t.Run(`apiJobHttp_notfound`, func(tt *testing.T) {
		testKarajo_apiJobHttp_notfound(tt, tdata, testClient)
	})
	t.Run(`apiJobLogs`, func(tt *testing.T) {
		testKarajo_apiJobHttpLogs(tt, tdata, testClient)
	})
	t.Run(`apiJobHttpPause`, func(tt *testing.T) {
		testKarajo_apiJobHttpPause(tt, tdata, testClient)
	})
	t.Run(`apiJobHttpResume`, func(tt *testing.T) {
		testKarajo_apiJobHttpResume(tt, tdata, testClient)
	})
}

func waitServerAlive(t *testing.T, cl *Client) {
	var (
		err error
	)

	for {
		_, _, err = cl.Get(`/`, nil, nil)
		if err != nil {
			t.Logf(`waitServerAlive: %s`, err)
			continue
		}
		return
	}
}

func testKarajo_apiEnvironment(t *testing.T, tdata *test.Data, cl *Client) {
	var (
		exp []byte = tdata.Output[`apiEnvironment.json`]

		gotEnv  *Environment
		httpJob *JobHttp
		job     *Job
		got     []byte
		err     error
	)

	gotEnv, err = testClient.Environment()
	if err != nil {
		t.Fatal(err)
	}

	// Clear the log and overwrite the LastRun to fixed time.
	for _, httpJob = range gotEnv.HttpJobs {
		httpJob.Log.Reset()
	}
	for _, job = range gotEnv.Jobs {
		job.Logs = nil
	}
	gotEnv.DirBase = `<REDACTED>`

	got, err = json.MarshalIndent(gotEnv, ``, `  `)
	if err != nil {
		t.Fatal(err)
	}
	test.Assert(t, `apiEnvironment`, string(exp), string(got))
}

func testKarajo_apiJobRun_success(t *testing.T, tdata *test.Data, cl *Client) {
	var (
		exp []byte = tdata.Output[`apiJobRun_success.json`]

		job  *Job
		data interface{}
		got  []byte
		err  error
	)

	job, err = testClient.JobRun(`/test-job-success`)
	if err != nil {
		data = err
	} else {
		data = job
	}

	got, err = json.MarshalIndent(data, ``, `  `)
	if err != nil {
		t.Fatal(err)
	}
	test.Assert(t, `apiJobRun_success`, string(exp), string(got))
}

func testKarajo_apiJobRun_notfound(t *testing.T, tdata *test.Data, cl *Client) {
	var (
		exp []byte = tdata.Output[`apiJobRun_notfound.json`]

		job  *Job
		data interface{}
		got  []byte
		err  error
	)

	job, err = testClient.JobRun(`/test-job-notfound`)
	if err != nil {
		data = err
	} else {
		data = job
	}

	got, err = json.MarshalIndent(data, ``, `  `)
	if err != nil {
		t.Fatal(err)
	}
	test.Assert(t, `apiJobRun_notfound`, string(exp), string(got))
}

func testKarajo_apiJobLog(t *testing.T, tdata *test.Data, cl *Client) {
	var (
		exp []byte = tdata.Output[`apiJobLog.json`]

		joblog *JobLog
		data   interface{}
		got    []byte
		err    error
	)

	joblog, err = testClient.JobLog(`test-job-success`, 1)
	if err != nil {
		data = err
	} else {
		joblog.Content = []byte(`<REDACTED>`)
		data = joblog
	}

	got, err = json.MarshalIndent(data, ``, `  `)
	if err != nil {
		t.Fatal(err)
	}

	test.Assert(t, `apiJobLog`, string(exp), string(got))
}

func testKarajo_apiJobHttp_success(t *testing.T, tdata *test.Data, cl *Client) {
	var (
		exp []byte = tdata.Output[`apiJobHttp_success.json`]

		gotJob *JobHttp
		got    []byte
		err    error
	)

	gotJob, err = testClient.JobHttp(`test_success`)
	if err != nil {
		t.Fatal(err)
	}

	gotJob.Log.Reset()

	got, err = json.MarshalIndent(gotJob, ``, `  `)
	if err != nil {
		t.Fatal(err)
	}
	test.Assert(t, `apiJobHttp_success`, string(exp), string(got))
}

func testKarajo_apiJobHttp_notfound(t *testing.T, tdata *test.Data, cl *Client) {
	var (
		exp []byte = tdata.Output[`apiJobHttp_notfound.json`]

		data   interface{}
		gotJob *JobHttp
		got    []byte
		err    error
	)

	gotJob, err = testClient.JobHttp(`test_notfound`)
	if err != nil {
		data = err
	} else {
		if gotJob != nil {
			gotJob.Log.Reset()
		}
		data = gotJob
	}

	got, err = json.MarshalIndent(data, ``, `  `)
	if err != nil {
		t.Fatal(err)
	}
	test.Assert(t, `apiJobHttp_notfound`, string(exp), string(got))
}

func testKarajo_apiJobHttpLogs(t *testing.T, tdata *test.Data, cl *Client) {
	var (
		exp []byte = tdata.Output[`apiJobHttpLogs.json`]

		data interface{}
		logs []string
		got  []byte
		err  error
	)

	// Add dummy logs.
	testEnv.httpJobs[`test_success`].Log.Push(`The first log`)

	logs, err = testClient.JobHttpLogs(`test_success`)
	if err != nil {
		data = err
	} else {
		data = logs
	}

	got, err = json.MarshalIndent(data, ``, `  `)
	if err != nil {
		t.Fatal(err)
	}
	test.Assert(t, `apiJobHttpLogs`, string(exp), string(got))
}

func testKarajo_apiJobHttpPause(t *testing.T, tdata *test.Data, cl *Client) {
	var (
		exp []byte = tdata.Output[`apiJobHttpPause.json`]

		data interface{}
		job  *JobHttp
		got  []byte
		err  error
	)

	job, err = testClient.JobHttpPause(`test_success`)
	if err != nil {
		data = err
	} else {
		data = job
	}

	got, err = json.MarshalIndent(data, ``, `  `)
	if err != nil {
		t.Fatal(err)
	}
	test.Assert(t, `apiJobHttpPause`, string(exp), string(got))
}

func testKarajo_apiJobHttpResume(t *testing.T, tdata *test.Data, cl *Client) {
	var (
		exp []byte = tdata.Output[`apiJobHttpResume.json`]

		data interface{}
		job  *JobHttp
		got  []byte
		err  error
	)

	job, err = testClient.JobHttpResume(`test_success`)
	if err != nil {
		data = err
	} else {
		data = job
	}

	got, err = json.MarshalIndent(data, ``, `  `)
	if err != nil {
		t.Fatal(err)
	}
	test.Assert(t, `apiJobHttpResume`, string(exp), string(got))
}