// SPDX-FileCopyrightText: 2023 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

package karajo

import (
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"regexp"
	"testing"

	libhttp "github.com/shuLhan/share/lib/http"
	"github.com/shuLhan/share/lib/test"
)

func TestKarajo_apiAuthLogin(t *testing.T) {
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
			"Set-Cookie: karajo=.{32}?; Path=/; Max-Age=86400; HttpOnly\r\n\r\n",
		expSM: &sessionManager{
			value: map[string]*User{
				`<RANDOM>`: user,
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

		var reHeader = regexp.MustCompile(c.expResHeader)

		if !reHeader.Match(rawResp) {
			t.Fatalf(`response header: want %q got %q`, c.expResHeader, string(rawResp))
		}

		test.Assert(t, c.desc+`: session manager`, 1, len(k.sm.value))
	}
}

func TestKarajo_handleFSAuth(t *testing.T) {
	var (
		env = &Environment{
			Users: map[string]*User{},
		}
		k = &Karajo{
			env: env,
			sm:  newSessionManager(),
		}
	)

	t.Run(`WithUser`, func(tt *testing.T) {
		testHandleFSAuthWithUser(tt, k)
	})
	t.Run(`WithoutUser`, func(tt *testing.T) {
		testHandleFSAuthWithoutUser(tt, k)
	})

}

func testHandleFSAuthWithUser(t *testing.T, k *Karajo) {
	var (
		user = &User{
			Name:     `tester`,
			Password: `$2a$10$9XMRfqpnzY2421fwYm5dd.CidJf7dHHWIESeeNGXuajHRf.Lqzy7a`, // s3cret
		}
		cookie = &http.Cookie{
			Name:  cookieName,
			Value: `abcd`,
		}
	)

	k.env.Users[user.Name] = user
	k.sm.value[cookie.Value] = user

	type testCase struct {
		cookie *http.Cookie
		desc   string
		path   string
		exp    bool
	}

	var cases = []testCase{{
		desc: `to root without cookie`,
		path: `/`,
		exp:  true,
	}, {
		desc: `to login without cookie`,
		path: `/karajo/`,
		exp:  true,
	}, {
		desc:   `to login with cookie`,
		path:   `/karajo/`,
		exp:    false, // Request redirect to app.
		cookie: cookie,
	}, {
		desc: `to app without cookie`,
		path: `/karajo/app/index.html`,
		exp:  false,
	}, {
		desc: `to app with cookie not exist`,
		path: `/karajo/app/index.html`,
		exp:  false,
		cookie: &http.Cookie{
			Name:  cookieName,
			Value: `notexist`,
		},
	}, {
		desc:   `to app with cookie exist`,
		path:   `/karajo/app/index.html`,
		exp:    true,
		cookie: cookie,
	}, {
		desc: `to api without cookie`,
		path: `/karajo/api/environment`,
		exp:  false,
	}, {
		desc:   `to api with cookie`,
		path:   `/karajo/api/environment`,
		exp:    true,
		cookie: cookie,
	}}

	var (
		recordWriter = httptest.NewRecorder()
		httpReq      = &http.Request{
			URL:    &url.URL{},
			Header: http.Header{},
		}

		c   testCase
		got bool
	)
	for _, c = range cases {
		httpReq.URL.Path = c.path
		httpReq.Header.Del(`Cookie`)
		if c.cookie != nil {
			httpReq.AddCookie(c.cookie)
		}

		got = k.handleFSAuth(nil, recordWriter, httpReq)
		test.Assert(t, c.desc, c.exp, got)
	}
}

func testHandleFSAuthWithoutUser(t *testing.T, k *Karajo) {
	var cookie = &http.Cookie{
		Name:  cookieName,
		Value: `abcd`,
	}

	k.env.Users = map[string]*User{}

	type testCase struct {
		cookie *http.Cookie
		desc   string
		path   string
		exp    bool
	}

	var cases = []testCase{{
		desc: `to root without cookie`,
		path: `/`,
		exp:  true,
	}, {
		desc: `to login without cookie`,
		path: `/karajo/`,
		exp:  false, // Redirected to app.
	}, {
		desc:   `to login with cookie`,
		path:   `/karajo/`,
		exp:    false, // Redirected to app.
		cookie: cookie,
	}, {
		desc: `to app without cookie`,
		path: `/karajo/app/index.html`,
		exp:  true,
	}, {
		desc:   `to app with cookie`,
		path:   `/karajo/app/index.html`,
		exp:    true,
		cookie: cookie,
	}, {
		desc: `to api without cookie`,
		path: `/karajo/api/environment`,
		exp:  true,
	}}

	var (
		recordWriter = httptest.NewRecorder()
		httpReq      = &http.Request{
			URL:    &url.URL{},
			Header: http.Header{},
		}

		c   testCase
		got bool
	)
	for _, c = range cases {
		httpReq.URL.Path = c.path
		httpReq.Header.Del(`Cookie`)
		if c.cookie != nil {
			httpReq.AddCookie(c.cookie)
		}

		got = k.handleFSAuth(nil, recordWriter, httpReq)
		test.Assert(t, c.desc, c.exp, got)
	}

}
