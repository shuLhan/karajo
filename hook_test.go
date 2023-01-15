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

// TestHook_handleHttp test Hook's Call with HTTP request.
func TestHook_handleHttp(t *testing.T) {
	var (
		testBaseDir = t.TempDir()
		env         = Environment{
			DirBase: testBaseDir,
			Secret:  `s3cret`,
		}
		hook = Hook{
			JobBase: JobBase{
				Name: `Test hook handle HTTP`,
			},
			Path:   `/test-hook-handle-http`,
			Secret: `s3cret`,
			Call: func(hlog io.Writer, _ *libhttp.EndpointRequest) error {
				fmt.Fprintf(hlog, `Output from Call`)
				return nil
			},
		}

		tdata *test.Data
		err   error
	)

	tdata, err = test.LoadData(`testdata/hook_handleHttp_test.txt`)
	if err != nil {
		t.Fatal(err)
	}

	err = env.init()
	if err != nil {
		t.Fatal(err)
	}

	err = hook.init(&env, hook.Name)
	if err != nil {
		t.Fatal(err)
	}

	var (
		hookReq = HookRequest{
			Epoch: testTimeNow.Unix(),
		}
		epr = libhttp.EndpointRequest{
			HttpRequest: &http.Request{
				Header: http.Header{},
			},
		}
		sign string
	)

	epr.RequestBody, err = json.Marshal(&hookReq)
	if err != nil {
		t.Fatal(err)
	}

	sign = Sign(epr.RequestBody, []byte(hook.Secret))
	epr.HttpRequest.Header.Set(HeaderNameXKarajoSign, sign)

	var (
		buf bytes.Buffer
		got []byte
		exp []byte
	)

	got, err = hook.handleHttp(&epr)
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

	<-hook.finished

	hook.Lock()
	got, err = json.MarshalIndent(&hook, ``, `  `)
	if err != nil {
		hook.Unlock()
		t.Fatal(err)
	}
	hook.Unlock()

	exp = tdata.Output[`hook_after.json`]
	test.Assert(t, `TestHook_Call`, string(exp), string(got))
}

// TestHook_Start test Hook's Call with timer.
func TestHook_Start(t *testing.T) {
	var (
		testBaseDir = t.TempDir()
		env         = Environment{
			DirBase: testBaseDir,
			Secret:  `s3cret`,
		}
		hook = Hook{
			JobBase: JobBase{
				Name:     `Test hook timer`,
				Interval: time.Minute,
			},
			Path:   `/test-hook-timer`,
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

	tdata, err = test.LoadData(`testdata/hook_Start_test.txt`)
	if err != nil {
		t.Fatal(err)
	}

	err = env.init()
	if err != nil {
		t.Fatal(err)
	}

	err = hook.init(&env, hook.Name)
	if err != nil {
		t.Fatal(err)
	}

	go hook.Start()
	defer hook.Stop()

	<-hook.finished

	hook.Lock()
	got, err = json.MarshalIndent(&hook, ``, `  `)
	if err != nil {
		hook.Unlock()
		t.Fatal(err)
	}
	hook.Unlock()

	exp = tdata.Output[`hook_after.json`]
	test.Assert(t, `TestHook_Call`, string(exp), string(got))
}
