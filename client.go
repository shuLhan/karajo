// SPDX-FileCopyrightText: 2023 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

package karajo

import (
	"encoding/json"
	"fmt"
	"net/url"

	libhttp "github.com/shuLhan/share/lib/http"
)

// Client HTTP client for Karajo server.
type Client struct {
	*libhttp.Client
}

// NewClient create new HTTP client.
func NewClient(opts *libhttp.ClientOptions) (cl *Client) {
	cl = &Client{
		Client: libhttp.NewClient(opts),
	}
	return cl
}

// Environment get the server environment.
func (cl *Client) Environment() (env *Environment, err error) {
	var (
		logp = `Environment`

		res     *libhttp.EndpointResponse
		resBody []byte
	)

	_, resBody, err = cl.Get(apiEnvironment, nil, nil)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}

	env = &Environment{}
	res = &libhttp.EndpointResponse{
		Data: env,
	}
	err = json.Unmarshal(resBody, res)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}
	if res.Code != 200 {
		res.Data = nil
		return nil, res
	}
	return env, nil
}

// JobHttp get JobHttp detail by its ID.
func (cl *Client) JobHttp(id string) (httpJob *JobHttp, err error) {
	var (
		logp   = `HttpJob`
		params = url.Values{}

		res     *libhttp.EndpointResponse
		resBody []byte
	)

	params.Set(`id`, id)

	_, resBody, err = cl.Get(apiJob, nil, params)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}

	httpJob = &JobHttp{}
	res = &libhttp.EndpointResponse{
		Data: httpJob,
	}
	err = json.Unmarshal(resBody, res)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}
	if res.Code != 200 {
		res.Data = nil
		return nil, res
	}
	return httpJob, nil
}
