// SPDX-FileCopyrightText: 2023 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

package karajo

import (
	"math/rand"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"testing"

	libhttp "github.com/shuLhan/share/lib/http"
	"github.com/shuLhan/share/lib/test"
)

func TestKarajo_apiAuthLogin(t *testing.T) {
	rand.Seed(42)

	var (
		user = &User{
			Name:     `tester`,
			Password: `$2a$10$9XMRfqpnzY2421fwYm5dd.CidJf7dHHWIESeeNGXuajHRf.Lqzy7a`, // s3cret

		}
		env = &Environment{
			Users: map[string]*User{
				user.Name: user,
			},
		}
		k = &Karajo{
			env: env,
			sm:  newSessionManager(),
		}
	)

	type testCase struct {
		expSM        *sessionManager
		desc         string
		name         string
		pass         string
		expError     string
		expResBody   string
		expResHeader string
	}

	var cases = []testCase{{
		desc:     `with empty name and password`,
		expError: errAuthLogin.Error(),
	}, {
		desc:     `with empty password`,
		name:     user.Name,
		expError: errAuthLogin.Message,
	}, {
		desc:     `with unknown user name`,
		name:     `unknown`,
		pass:     `notempty`,
		expError: errAuthLogin.Message,
	}, {
		desc:     `with invalid password`,
		name:     user.Name,
		pass:     `invalid`,
		expError: errAuthLogin.Message,
	}, {
		desc:       `with valid name and password`,
		name:       user.Name,
		pass:       `s3cret`,
		expResBody: `{"code":200}`,
		expResHeader: "HTTP/1.1 200 OK\r\n" +
			"Connection: close\r\n" +
			"Set-Cookie: karajo=ASG5Ohg1l0CMEefBrPrV9QtazJoL6uax; Path=/; Max-Age=86400; HttpOnly\r\n\r\n",
		expSM: &sessionManager{
			value: map[string]*User{
				`ASG5Ohg1l0CMEefBrPrV9QtazJoL6uax`: user,
			},
		},
	}}

	var (
		testRecorder = httptest.NewRecorder()
		epr          = &libhttp.EndpointRequest{
			HttpWriter: testRecorder,
			HttpRequest: &http.Request{
				Form: url.Values{},
			},
		}

		c        testCase
		respBody []byte
		rawResp  []byte
		err      error
		httpResp *http.Response
	)

	for _, c = range cases {
		epr.HttpRequest.Form.Set(`name`, c.name)
		epr.HttpRequest.Form.Set(`password`, c.pass)

		respBody, err = k.apiAuthLogin(epr)
		if err != nil {
			test.Assert(t, c.desc+`: error`, c.expError, err.Error())
			continue
		}

		test.Assert(t, c.desc+`: respBody`, c.expResBody, string(respBody))

		httpResp = testRecorder.Result()
		rawResp, err = httputil.DumpResponse(httpResp, false)
		if err != nil {
			t.Fatal(err)
		}

		test.Assert(t, c.desc+`: response header`, c.expResHeader, string(rawResp))

		test.Assert(t, c.desc+`: session manager`, c.expSM, k.sm)
	}
}
