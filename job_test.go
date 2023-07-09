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
	"time"

	libhttp "github.com/shuLhan/share/lib/http"
	"github.com/shuLhan/share/lib/test"
)

func TestJob_authGithub(t *testing.T) {
	type testCase struct {
		headers  http.Header
		desc     string
		expError string
		reqbody  []byte
	}

	var (
		jhook = Job{
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
		expError: fmt.Sprintf(`authGithub: %s`, ErrJobForbidden),
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

func TestJob_authSourcehut(t *testing.T) {
	type testCase struct {
		headers  http.Header
		desc     string
		expError string
		reqbody  []byte
	}

	var (
		jhook = Job{
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
		expError: fmt.Sprintf(`authSourcehut: %s`, ErrJobForbidden),
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

func TestJob_authHmacSha256(t *testing.T) {
	type testCase struct {
		headers  http.Header
		desc     string
		expError string
		reqbody  []byte
	}

	var (
		jhook = Job{
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
		expError: fmt.Sprintf(`authHmacSha256: %s`, ErrJobForbidden),
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

	go job.Start()
	defer job.Stop()

	var (
		jobReq = JobHttpRequest{
			Epoch: TimeNow().Unix(),
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

	<-job.finishq

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

// TestJob_startInterval_Call test Job's Call with Interval.
func TestJob_startInterval_Call(t *testing.T) {
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

	<-job.finishq

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
