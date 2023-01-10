// SPDX-FileCopyrightText: 2023 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

package karajo

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	libhttp "github.com/shuLhan/share/lib/http"
)

// Client HTTP client for Karajo server.
type Client struct {
	*libhttp.Client
	opts ClientOptions
}

// NewClient create new HTTP client.
func NewClient(opts ClientOptions) (cl *Client) {
	cl = &Client{
		opts:   opts,
		Client: libhttp.NewClient(&opts.ClientOptions),
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
		logp   = `JobHttp`
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

// JobHttpLogs get the job logs by its ID.
func (cl *Client) JobHttpLogs(id string) (logs []string, err error) {
	var (
		logp   = `JobHttpLogs`
		params = url.Values{}

		res     *libhttp.EndpointResponse
		resBody []byte
	)

	params.Set(`id`, id)

	_, resBody, err = cl.Get(apiJobLogs, nil, params)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}

	res = &libhttp.EndpointResponse{
		Data: &logs,
	}

	err = json.Unmarshal(resBody, res)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}
	if res.Code != 200 {
		res.Data = nil
		return nil, res
	}
	return logs, nil
}

// JobHttpPause pause the HTTP job by its ID.
func (cl *Client) JobHttpPause(id string) (jobHttp *JobHttp, err error) {
	var (
		logp   = `JobHttpPause`
		params = url.Values{}
		header = http.Header{}

		sign    string
		res     *libhttp.EndpointResponse
		resBody []byte
	)

	params.Set(`id`, id)

	sign = Sign([]byte(params.Encode()), []byte(cl.opts.Secret))
	header.Set(HeaderNameXKarajoSign, sign)

	_, resBody, err = cl.Post(apiJobPause, header, params)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}

	res = &libhttp.EndpointResponse{
		Data: &jobHttp,
	}

	err = json.Unmarshal(resBody, res)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}
	if res.Code != 200 {
		res.Data = nil
		return nil, res
	}
	return jobHttp, nil
}

// JobHttpResume resume the HTTP job by its ID.
func (cl *Client) JobHttpResume(id string) (jobHttp *JobHttp, err error) {
	var (
		logp   = `JobHttpResume`
		params = url.Values{}
		header = http.Header{}

		sign    string
		res     *libhttp.EndpointResponse
		resBody []byte
	)

	params.Set(`id`, id)

	sign = Sign([]byte(params.Encode()), []byte(cl.opts.Secret))
	header.Set(HeaderNameXKarajoSign, sign)

	_, resBody, err = cl.Post(apiJobResume, header, params)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}

	res = &libhttp.EndpointResponse{
		Data: &jobHttp,
	}

	err = json.Unmarshal(resBody, res)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}
	if res.Code != 200 {
		res.Data = nil
		return nil, res
	}
	return jobHttp, nil
}
