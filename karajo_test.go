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
	for _, httpJob = range testEnv.HttpJobs {
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
		testKarajo_apiEnv(tt, tdata)
	})

	t.Run(`apiJobPause`, func(tt *testing.T) {
		testKarajo_apiJobPause(tt, tdata)
	})
	t.Run(`apiJobResume`, func(tt *testing.T) {
		testKarajo_apiJobResume(tt, tdata)
	})

	t.Run(`apiJobRun_success`, func(tt *testing.T) {
		testKarajo_apiJobRun_success(tt, tdata)
	})

	t.Run(`apiJobRun_notfound`, func(tt *testing.T) {
		testKarajo_apiJobRun_notfound(tt, tdata)
	})
	t.Run(`apiJobLog`, func(tt *testing.T) {
		testKarajo_apiJobLog(tt, tdata)
	})

	t.Run(`apiJobHttp_success`, func(tt *testing.T) {
		testKarajo_apiJobHttp_success(tt, tdata)
	})
	t.Run(`apiJobHttp_notfound`, func(tt *testing.T) {
		testKarajo_apiJobHttp_notfound(tt, tdata)
	})
	t.Run(`apiJobHttpLog`, func(tt *testing.T) {
		testKarajo_apiJobHttpLog(tt, tdata)
	})
	t.Run(`apiJobHttpPause`, func(tt *testing.T) {
		testKarajo_apiJobHttpPause(tt, tdata)
	})
	t.Run(`apiJobHttpResume`, func(tt *testing.T) {
		testKarajo_apiJobHttpResume(tt, tdata)
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

func testKarajo_apiEnv(t *testing.T, tdata *test.Data) {
	var (
		exp []byte = tdata.Output[`apiEnv.json`]

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

func testKarajo_apiJobPause(t *testing.T, tdata *test.Data) {
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

func testKarajo_apiJobRun_success(t *testing.T, tdata *test.Data) {
	var (
		exp []byte = tdata.Output[`apiJobRun_success.json`]

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

	test.Assert(t, `apiJobRun_success`, string(exp), string(got))
}

func testKarajo_apiJobRun_notfound(t *testing.T, tdata *test.Data) {
	var (
		exp []byte = tdata.Output[`apiJobRun_notfound.json`]

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
	test.Assert(t, `apiJobRun_notfound`, string(exp), string(got))
}

func testKarajo_apiJobLog(t *testing.T, tdata *test.Data) {
	var (
		exp []byte = tdata.Output[`apiJobLog.json`]

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

func testKarajo_apiJobResume(t *testing.T, tdata *test.Data) {
	var (
		exp []byte = tdata.Output[`apiJobResume.json`]

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

func testKarajo_apiJobHttp_success(t *testing.T, tdata *test.Data) {
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

	got, err = json.MarshalIndent(gotJob, ``, `  `)
	if err != nil {
		t.Fatal(err)
	}
	test.Assert(t, `apiJobHttp_success`, string(exp), string(got))
}

func testKarajo_apiJobHttp_notfound(t *testing.T, tdata *test.Data) {
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
		data = gotJob
	}

	got, err = json.MarshalIndent(data, ``, `  `)
	if err != nil {
		t.Fatal(err)
	}
	test.Assert(t, `apiJobHttp_notfound`, string(exp), string(got))
}

func testKarajo_apiJobHttpLog(t *testing.T, tdata *test.Data) {
	var (
		id      = `test_success`
		jobHttp = testEnv.jobHttp(id)

		data    interface{}
		jlog    *JobLog
		gotJlog *JobLog
		exp     []byte
		got     []byte
		err     error
	)

	// Add dummy logs.
	jobHttp.lastCounter++
	jlog = newJobLog(&jobHttp.JobBase)
	_, _ = jlog.Write([]byte("The first log\n"))
	jobHttp.Logs = append(jobHttp.Logs, jlog)
	_ = jlog.flush()

	gotJlog, err = testClient.JobHttpLog(id, int(jobHttp.lastCounter))
	if err != nil {
		data = err
	} else {
		data = gotJlog
	}

	got, err = json.MarshalIndent(data, ``, `  `)
	if err != nil {
		t.Fatal(err)
	}

	exp = tdata.Output[`apiJobHttpLog.json`]
	test.Assert(t, `apiJobHttpLog`, string(exp), string(got))
}

func testKarajo_apiJobHttpPause(t *testing.T, tdata *test.Data) {
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

func testKarajo_apiJobHttpResume(t *testing.T, tdata *test.Data) {
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
