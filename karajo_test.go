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

	testEnv    *Env
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

func TestKarajoAPIs(t *testing.T) {
	var (
		tdata   *test.Data
		httpJob *JobHTTP
		karajo  *Karajo
		err     error
	)

	tdata, err = test.LoadData(`testdata/api_test.txt`)
	if err != nil {
		t.Fatal(err)
	}

	testEnv, err = ParseEnv(tdata.Input[`test.conf`])
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
	for _, httpJob = range testEnv.HTTPJobs {
		httpJob.LastRun = testTimeNow
	}

	go func() {
		var err = karajo.Start()
		if err != nil {
			log.Fatal(err)
		}
	}()

	t.Cleanup(func() {
		var err = karajo.Stop()
		if err != nil {
			log.Fatal(err)
		}
	})

	var clientOpts = ClientOptions{
		ClientOptions: libhttp.ClientOptions{
			ServerUrl: fmt.Sprintf(`http://%s`, testEnv.ListenAddress),
		},
		Secret: `s3cret`,
	}
	testClient = NewClient(clientOpts)
	waitServerAlive(t, testClient)

	t.Run(`apiEnv`, func(tt *testing.T) {
		testKarajoAPIEnv(tt, tdata)
	})

	t.Run(`apiJobPause`, func(tt *testing.T) {
		testKarajoAPIJobPause(tt, tdata)
	})
	t.Run(`apiJobResume`, func(tt *testing.T) {
		testKarajoAPIJobResume(tt, tdata)
	})

	t.Run(`apiJobRunSuccess`, func(tt *testing.T) {
		testKarajoAPIJobRunSuccess(tt, tdata)
	})

	t.Run(`apiJobRunNotfound`, func(tt *testing.T) {
		testKarajoAPIJobRunNotfound(tt, tdata)
	})
	t.Run(`apiJobLog`, func(tt *testing.T) {
		testKarajoAPIJobLog(tt, tdata)
	})

	t.Run(`apiJobHTTPSuccess`, func(tt *testing.T) {
		testKarajoAPIJobHTTPSuccess(tt, tdata)
	})
	t.Run(`apiJobHTTPNotfound`, func(tt *testing.T) {
		testKarajoAPIJobHTTPNotfound(tt, tdata)
	})
	t.Run(`apiJobHTTPLog`, func(tt *testing.T) {
		testKarajoAPIJobHTTPLog(tt, tdata)
	})
	t.Run(`apiJobHTTPPause`, func(tt *testing.T) {
		testKarajoAPIJobHTTPPause(tt, tdata)
	})
	t.Run(`apiJobHTTPResume`, func(tt *testing.T) {
		testKarajoAPIJobHTTPResume(tt, tdata)
	})
}

func waitServerAlive(t *testing.T, cl *Client) {
	var (
		err error
	)

	for {
		_, _, err = cl.Client.Get(`/`, nil, nil)
		if err != nil {
			t.Logf(`waitServerAlive: %s`, err)
			continue
		}
		return
	}
}

func testKarajoAPIEnv(t *testing.T, tdata *test.Data) {
	var (
		exp = tdata.Output[`apiEnv.json`]

		gotEnv *Env
		job    *JobExec
		got    []byte
		err    error
	)

	gotEnv, err = testClient.Env()
	if err != nil {
		t.Fatal(err)
	}

	for _, job = range gotEnv.ExecJobs {
		job.Logs = nil
	}
	gotEnv.DirBase = `<REDACTED>`

	got, err = json.MarshalIndent(gotEnv, ``, `  `)
	if err != nil {
		t.Fatal(err)
	}
	test.Assert(t, `apiEnv`, string(exp), string(got))
}

func testKarajoAPIJobPause(t *testing.T, tdata *test.Data) {
	var (
		job  *JobExec
		data interface{}
		exp  []byte
		got  []byte
		err  error
	)

	job, err = testClient.JobPause(`test_job_success`)
	if err != nil {
		data = err
	} else {
		job.Logs = nil
		data = job
	}

	got, err = json.MarshalIndent(data, ``, `  `)
	if err != nil {
		t.Fatal(err)
	}

	exp = tdata.Output[`apiJobPause.json`]
	test.Assert(t, `apiJobPause`, string(exp), string(got))

	// Try triggering the JobExec to run...

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

	exp = tdata.Output[`apiJobPause_run.json`]
	test.Assert(t, `apiJobPause_run`, string(exp), string(got))
}

func testKarajoAPIJobRunSuccess(t *testing.T, tdata *test.Data) {
	var (
		exp = tdata.Output[`apiJobRun_success.json`]

		job  *JobExec
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

	test.Assert(t, `apiJobRunSuccess`, string(exp), string(got))
}

func testKarajoAPIJobRunNotfound(t *testing.T, tdata *test.Data) {
	var (
		exp = tdata.Output[`apiJobRun_notfound.json`]

		job  *JobExec
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
	test.Assert(t, `apiJobRunNotfound`, string(exp), string(got))
}

func testKarajoAPIJobLog(t *testing.T, tdata *test.Data) {
	var (
		exp = tdata.Output[`apiJobLog.json`]

		joblog *JobLog
		expErr string
		got    []byte
		err    error
	)

	_, err = testClient.JobLog(`test-job-success`, 1)
	expErr = `job ID test-job-success not found`
	test.Assert(t, `With invalid job ID`, expErr, err.Error())

	_, err = testClient.JobLog(`test_job_success`, -1)
	expErr = `log #-1 not found`
	test.Assert(t, `With invalid JobLog counter`, expErr, err.Error())

	joblog, err = testClient.JobLog(`test_job_success`, 1)
	if err != nil {
		t.Fatalf(`want no error, got %q`, err)
	}

	got, err = json.MarshalIndent(joblog, ``, `  `)
	if err != nil {
		t.Fatal(err)
	}

	test.Assert(t, `apiJobLog.json`, string(exp), string(got))
}

func testKarajoAPIJobResume(t *testing.T, tdata *test.Data) {
	var (
		exp = tdata.Output[`apiJobResume.json`]

		job  *JobExec
		data interface{}
		got  []byte
		err  error
	)

	job, err = testClient.JobResume(`test_job_success`)
	if err != nil {
		data = err
	} else {
		job.Logs = nil
		data = job
	}

	got, err = json.MarshalIndent(data, ``, `  `)
	if err != nil {
		t.Fatal(err)
	}

	test.Assert(t, `apiJobResume`, string(exp), string(got))
}

func testKarajoAPIJobHTTPSuccess(t *testing.T, tdata *test.Data) {
	var (
		exp = tdata.Output[`apiJobHTTP_success.json`]

		gotJob *JobHTTP
		got    []byte
		err    error
	)

	gotJob, err = testClient.JobHTTP(`test_success`)
	if err != nil {
		t.Fatal(err)
	}

	got, err = json.MarshalIndent(gotJob, ``, `  `)
	if err != nil {
		t.Fatal(err)
	}
	test.Assert(t, `apiJobHTTPSuccess`, string(exp), string(got))
}

func testKarajoAPIJobHTTPNotfound(t *testing.T, tdata *test.Data) {
	var (
		exp = tdata.Output[`apiJobHTTP_notfound.json`]

		data   interface{}
		gotJob *JobHTTP
		got    []byte
		err    error
	)

	gotJob, err = testClient.JobHTTP(`test_notfound`)
	if err != nil {
		data = err
	} else {
		data = gotJob
	}

	got, err = json.MarshalIndent(data, ``, `  `)
	if err != nil {
		t.Fatal(err)
	}
	test.Assert(t, `apiJobHTTPNotfound`, string(exp), string(got))
}

func testKarajoAPIJobHTTPLog(t *testing.T, tdata *test.Data) {
	var (
		id      = `test_success`
		jobHTTP = testEnv.jobHTTP(id)

		data    interface{}
		jlog    *JobLog
		gotJlog *JobLog
		exp     []byte
		got     []byte
		err     error
	)

	// Add dummy log.
	jlog = jobHTTP.JobBase.newLog()
	_, _ = jlog.Write([]byte("The first log\n"))
	_ = jlog.flush()

	gotJlog, err = testClient.JobHTTPLog(id, int(jobHTTP.counter))
	if err != nil {
		data = err
	} else {
		data = gotJlog
	}

	got, err = json.MarshalIndent(data, ``, `  `)
	if err != nil {
		t.Fatal(err)
	}

	exp = tdata.Output[`apiJobHTTPLog.json`]
	test.Assert(t, `apiJobHTTPLog`, string(exp), string(got))
}

func testKarajoAPIJobHTTPPause(t *testing.T, tdata *test.Data) {
	var (
		exp = tdata.Output[`apiJobHTTPPause.json`]

		data interface{}
		job  *JobHTTP
		got  []byte
		err  error
	)

	job, err = testClient.JobHTTPPause(`test_success`)
	if err != nil {
		data = err
	} else {
		data = job
	}

	got, err = json.MarshalIndent(data, ``, `  `)
	if err != nil {
		t.Fatal(err)
	}
	test.Assert(t, `apiJobHTTPPause`, string(exp), string(got))
}

func testKarajoAPIJobHTTPResume(t *testing.T, tdata *test.Data) {
	var (
		exp = tdata.Output[`apiJobHTTPResume.json`]

		data interface{}
		job  *JobHTTP
		got  []byte
		err  error
	)

	job, err = testClient.JobHTTPResume(`test_success`)
	if err != nil {
		data = err
	} else {
		data = job
	}

	got, err = json.MarshalIndent(data, ``, `  `)
	if err != nil {
		t.Fatal(err)
	}
	test.Assert(t, `apiJobHTTPResume`, string(exp), string(got))
}
