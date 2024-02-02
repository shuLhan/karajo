// SPDX-FileCopyrightText: 2023 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

package karajo

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	libhttp "github.com/shuLhan/share/lib/http"
	"github.com/shuLhan/share/lib/test"
)

func TestJobExec_authGithub(t *testing.T) {
	type testCase struct {
		headers  http.Header
		desc     string
		expError string
		reqbody  []byte
	}

	var (
		jhook = JobExec{
			Secret:   `s3cret`,
			AuthKind: JobAuthKindGithub,
		}

		payload = []byte(`_karajo_sign=123`)
		sign256 = Sign(payload, []byte(jhook.Secret))
		sign1   = signHmacSha1(payload, []byte(jhook.Secret))
	)

	var cases = []testCase{{
		desc: `with valid sha256 signature`,
		headers: http.Header{
			githubHeaderSign256: []string{`sha256=` + sign256},
		},
		reqbody: payload,
	}, {
		desc: `with valid sha1 signature`,
		headers: http.Header{
			githubHeaderSign: []string{sign1},
		},
		reqbody: payload,
	}, {
		desc: `with invalid payload`,
		headers: http.Header{
			githubHeaderSign256: []string{`sha256=` + sign256},
		},
		reqbody:  []byte(`_karajo_sign=1234`),
		expError: fmt.Sprintf(`authGithub: %s`, errJobForbidden.Error()),
	}}

	var (
		c   testCase
		err error
	)

	for _, c = range cases {
		var gotError string

		err = jhook.authGithub(c.headers, c.reqbody)
		if err != nil {
			gotError = err.Error()
		}

		test.Assert(t, c.desc, c.expError, gotError)
	}
}

func TestJobExec_authSourcehut(t *testing.T) {
	type testCase struct {
		headers  http.Header
		desc     string
		expError string
		reqbody  []byte
	}

	var (
		jhook = JobExec{
			AuthKind: JobAuthKindSourcehut,
		}
		payload = []byte(`_karajo_sign=123`)
		nonce   = `4`

		msg     bytes.Buffer
		pubKey  ed25519.PublicKey
		privKey ed25519.PrivateKey
		err     error
	)

	pubKey, privKey, err = ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}

	msg.Write(payload)
	msg.WriteString(nonce)

	var (
		sign    = ed25519.Sign(privKey, msg.Bytes())
		signb64 = base64.StdEncoding.EncodeToString(sign)
	)

	var cases = []testCase{{
		desc: `with valid signature`,
		headers: http.Header{
			sourcehutHeaderSign:  []string{signb64},
			sourcehutHeaderNonce: []string{nonce},
		},
		reqbody: payload,
	}, {
		desc: `with invalid payload`,
		headers: http.Header{
			sourcehutHeaderSign:  []string{signb64},
			sourcehutHeaderNonce: []string{nonce},
		},
		reqbody:  []byte(`_karajo_sign=1234`),
		expError: fmt.Sprintf(`authSourcehut: %s`, errJobForbidden.Error()),
	}}

	var (
		c testCase
	)

	for _, c = range cases {
		var gotError string

		err = jhook.authSourcehut(c.headers, c.reqbody, pubKey)
		if err != nil {
			gotError = err.Error()
		}

		test.Assert(t, c.desc, c.expError, gotError)
	}
}

func TestJobExec_authHmacSha256(t *testing.T) {
	type testCase struct {
		headers  http.Header
		desc     string
		expError string
		reqbody  []byte
	}

	var (
		jhook = JobExec{
			AuthKind:   JobAuthKindHmacSha256,
			Secret:     `s3cret`,
			HeaderSign: HeaderNameXKarajoSign,
		}

		payload = []byte(`_karajo_sign=123`)
		sign256 = Sign(payload, []byte(jhook.Secret))
	)

	var cases = []testCase{{
		desc: `with valid signature`,
		headers: http.Header{
			HeaderNameXKarajoSign: []string{sign256},
		},
		reqbody: payload,
	}, {
		desc: `with invalid payload`,
		headers: http.Header{
			HeaderNameXKarajoSign: []string{sign256},
		},
		reqbody:  []byte(`_karajo_sign=1234`),
		expError: fmt.Sprintf(`authHmacSha256: %s`, errJobForbidden.Error()),
	}}

	var (
		c   testCase
		err error
	)

	for _, c = range cases {
		var gotError string

		err = jhook.authHmacSha256(c.headers, c.reqbody)
		if err != nil {
			gotError = err.Error()
		}

		test.Assert(t, c.desc, c.expError, gotError)
	}
}

// TestJobExec_handleHTTP test JobExec Call with HTTP request.
func TestJobExec_handleHTTP(t *testing.T) {
	var (
		testBaseDir = t.TempDir()
		env         = Env{
			DirBase: testBaseDir,
			Secret:  `s3cret`,
		}
		job = JobExec{
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
		logq = make(chan *JobLog)

		tdata *test.Data
		err   error
	)

	tdata, err = test.LoadData(`testdata/job_handleHTTP_test.txt`)
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

	var jobq = make(chan struct{}, env.MaxJobRunning)

	go job.Start(jobq, logq)
	t.Cleanup(job.Stop)

	var (
		jobReq = JobHTTPRequest{
			Epoch: timeNow().Unix(),
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
	epr.HttpRequest.Header.Set(job.HeaderSign, sign)

	var (
		buf bytes.Buffer
		got []byte
		exp []byte
	)

	got, err = job.handleHTTP(&epr)
	if err != nil {
		t.Fatal(err)
	}

	err = json.Indent(&buf, got, ``, `  `)
	if err != nil {
		t.Fatal(err)
	}

	got = buf.Bytes()
	exp = tdata.Output[`handleHTTP_response.json`]
	test.Assert(t, `handleHTTP_response`, string(exp), string(got))

	<-logq

	job.Lock()
	got, err = json.MarshalIndent(&job, ``, `  `)
	if err != nil {
		job.Unlock()
		t.Fatal(err)
	}
	job.Unlock()

	exp = tdata.Output[`job_after.json`]
	test.Assert(t, `job_after`, string(exp), string(got))
}

func TestJobExecCall(t *testing.T) {
	var (
		testBaseDir = t.TempDir()
		env         = Env{
			DirBase: testBaseDir,
			Secret:  `s3cret`,
		}
		job = JobExec{
			JobBase: JobBase{
				Name: `Test job timer`,
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

	tdata, err = test.LoadData(`testdata/job_exec_call_test.txt`)
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

	job.jobq = make(chan struct{}, env.MaxJobRunning)
	job.logq = make(chan *JobLog)

	job.run(nil)

	job.Lock()
	got, err = json.MarshalIndent(&job, ``, `  `)
	if err != nil {
		job.Unlock()
		t.Fatal(err)
	}
	job.Unlock()

	exp = tdata.Output[`job_after.json`]
	test.Assert(t, `TestJobExecCall`, string(exp), string(got))
}
