// SPDX-FileCopyrightText: 2023 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

package karajo

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strconv"

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

// Hook trigger the Hook by its path.
func (cl *Client) Hook(hookPath string) (hook *Hook, err error) {
	var (
		logp        = `Hook`
		timeNow     = TimeNow()
		apiHookPath = path.Join(apiHook, hookPath)
		header      = http.Header{}

		req = HookRequest{
			Epoch: timeNow.Unix(),
		}

		httpRes *http.Response
		res     *libhttp.EndpointResponse
		sign    string
		reqBody []byte
		resBody []byte
	)

	reqBody, err = json.Marshal(&req)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}

	sign = Sign(reqBody, []byte(cl.opts.Secret))
	header.Set(HeaderNameXKarajoSign, sign)

	httpRes, resBody, err = cl.PostJSON(apiHookPath, header, &req)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}
	if httpRes.StatusCode == http.StatusNotFound {
		return nil, errHookNotFound(hookPath)
	}

	res = &libhttp.EndpointResponse{
		Data: &hook,
	}
	err = json.Unmarshal(resBody, res)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}
	if res.Code == 200 {
		return hook, nil
	}
	res.Data = nil
	return nil, res
}

// HookLog get the Hook log by its ID and counter.
func (cl *Client) HookLog(hookID string, counter int) (hooklog *HookLog, err error) {
	var (
		logp   = `HookLog`
		params = url.Values{}

		res     *libhttp.EndpointResponse
		resBody []byte
	)

	params.Set(paramNameID, hookID)
	params.Set(paramNameCounter, strconv.Itoa(counter))

	_, resBody, err = cl.Get(apiHookLog, nil, params)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}

	res = &libhttp.EndpointResponse{
		Data: &hooklog,
	}
	err = json.Unmarshal(resBody, res)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}
	if res.Code == 200 {
		return hooklog, nil
	}
	res.Data = nil
	return nil, res
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
