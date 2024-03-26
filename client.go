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

	libhttp "git.sr.ht/~shulhan/pakakeh.go/lib/http"
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
		Client: libhttp.NewClient(opts.ClientOptions),
	}
	return cl
}

// Env get the server environment.
func (cl *Client) Env() (env *Env, err error) {
	var (
		logp      = `Env`
		clientReq = libhttp.ClientRequest{
			Path: apiEnv,
		}
		clientResp *libhttp.ClientResponse
	)

	clientResp, err = cl.Client.Get(clientReq)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}

	env = &Env{}
	var res = &libhttp.EndpointResponse{
		Data: env,
	}
	err = json.Unmarshal(clientResp.Body, res)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}
	if res.Code != 200 {
		res.Data = nil
		return nil, res
	}
	return env, nil
}

// JobExecCancel cancel the running JobExec by its ID.
func (cl *Client) JobExecCancel(id string) (job *JobExec, err error) {
	var (
		logp   = `JobExecCancel`
		now    = timeNow().Unix()
		params = url.Values{}
		header = http.Header{}
	)

	params.Set(paramNameKarajoEpoch, strconv.FormatInt(now, 10))
	params.Set(paramNameID, id)

	var body = params.Encode()
	var sign = Sign([]byte(body), []byte(cl.opts.Secret))

	header.Set(HeaderNameXKarajoSign, sign)

	var (
		clientReq = libhttp.ClientRequest{
			Path:   apiJobExecCancel,
			Header: header,
			Params: params,
		}
		clientResp *libhttp.ClientResponse
	)

	clientResp, err = cl.Client.PostForm(clientReq)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}

	job = &JobExec{}
	var res = &libhttp.EndpointResponse{
		Data: job,
	}

	err = json.Unmarshal(clientResp.Body, &res)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}
	if res.Code != http.StatusOK {
		res.Data = nil
		return nil, res
	}

	return job, nil
}

// JobExecPause pause the JobExec by its ID.
func (cl *Client) JobExecPause(id string) (job *JobExec, err error) {
	var (
		logp   = `JobExecPause`
		now    = timeNow().Unix()
		params = url.Values{}
		header = http.Header{}

		body string
		sign string
	)

	params.Set(paramNameKarajoEpoch, strconv.FormatInt(now, 10))
	params.Set(paramNameID, id)

	body = params.Encode()

	sign = Sign([]byte(body), []byte(cl.opts.Secret))
	header.Set(HeaderNameXKarajoSign, sign)

	var (
		clientReq = libhttp.ClientRequest{
			Path:   apiJobExecPause,
			Header: header,
			Params: params,
		}
		clientResp *libhttp.ClientResponse
	)

	clientResp, err = cl.Client.PostForm(clientReq)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}

	job = &JobExec{}
	var res = &libhttp.EndpointResponse{
		Data: job,
	}

	err = json.Unmarshal(clientResp.Body, &res)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}
	if res.Code != http.StatusOK {
		res.Data = nil
		return nil, res
	}

	return job, nil
}

// JobExecResume resume the JobExec execution by its ID.
func (cl *Client) JobExecResume(id string) (job *JobExec, err error) {
	var (
		logp   = `JobExecResume`
		now    = timeNow().Unix()
		params = url.Values{}
		header = http.Header{}
	)

	params.Set(paramNameKarajoEpoch, strconv.FormatInt(now, 10))
	params.Set(paramNameID, id)

	var body = params.Encode()
	var sign = Sign([]byte(body), []byte(cl.opts.Secret))
	header.Set(HeaderNameXKarajoSign, sign)

	var (
		clientReq = libhttp.ClientRequest{
			Path:   apiJobExecResume,
			Header: header,
			Params: params,
		}
		clientResp *libhttp.ClientResponse
	)

	clientResp, err = cl.Client.PostForm(clientReq)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}

	job = &JobExec{}
	var res = &libhttp.EndpointResponse{
		Data: job,
	}

	err = json.Unmarshal(clientResp.Body, &res)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}
	if res.Code != http.StatusOK {
		res.Data = nil
		return nil, res
	}

	return job, nil
}

// JobExecRun trigger the JobExec by its path.
func (cl *Client) JobExecRun(jobPath string) (job *JobExec, err error) {
	var (
		logp       = `JobExec`
		timeNow    = timeNow()
		apiJobPath = path.Join(apiJobExecRun, jobPath)
		header     = http.Header{}

		req = JobHTTPRequest{
			Epoch: timeNow.Unix(),
		}

		reqBody []byte
	)

	reqBody, err = json.Marshal(&req)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}

	var sign = Sign(reqBody, []byte(cl.opts.Secret))
	header.Set(HeaderNameXKarajoSign, sign)

	var (
		clientReq = libhttp.ClientRequest{
			Path:   apiJobPath,
			Header: header,
			Params: &req,
		}
		clientResp *libhttp.ClientResponse
	)

	clientResp, err = cl.Client.PostJSON(clientReq)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}
	if clientResp.HTTPResponse.StatusCode == http.StatusNotFound {
		return nil, errJobNotFound(jobPath)
	}

	var res = &libhttp.EndpointResponse{
		Data: &job,
	}
	err = json.Unmarshal(clientResp.Body, res)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}
	if res.Code >= 400 {
		return nil, res
	}
	return job, nil
}

// JobExecLog get the JobExec log by its ID and counter.
func (cl *Client) JobExecLog(jobID string, counter int) (joblog *JobLog, err error) {
	var (
		logp   = `JobExecLog`
		params = url.Values{}
	)

	params.Set(paramNameID, jobID)
	params.Set(paramNameCounter, strconv.Itoa(counter))

	var (
		clientReq = libhttp.ClientRequest{
			Path:   apiJobExecLog,
			Params: params,
		}
		clientResp *libhttp.ClientResponse
	)

	clientResp, err = cl.Client.Get(clientReq)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}

	var res = &libhttp.EndpointResponse{
		Data: &joblog,
	}
	err = json.Unmarshal(clientResp.Body, res)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}
	if res.Code == 200 {
		return joblog, nil
	}
	res.Data = nil
	return nil, res
}

// JobHTTP get JobHTTP detail by its ID.
func (cl *Client) JobHTTP(id string) (httpJob *JobHTTP, err error) {
	var (
		logp   = `JobHTTP`
		params = url.Values{}
	)

	params.Set(`id`, id)

	var (
		clientReq = libhttp.ClientRequest{
			Path:   apiJobHTTP,
			Params: params,
		}
		clientResp *libhttp.ClientResponse
	)

	clientResp, err = cl.Client.Get(clientReq)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}

	httpJob = &JobHTTP{}
	var res = &libhttp.EndpointResponse{
		Data: httpJob,
	}
	err = json.Unmarshal(clientResp.Body, res)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}
	if res.Code != 200 {
		res.Data = nil
		return nil, res
	}
	return httpJob, nil
}

// JobHTTPLog get the job logs by its ID.
func (cl *Client) JobHTTPLog(id string, counter int) (jlog *JobLog, err error) {
	var (
		logp   = `JobHTTPLog`
		params = url.Values{}
	)

	params.Set(paramNameID, id)
	params.Set(paramNameCounter, strconv.Itoa(counter))

	var (
		clientReq = libhttp.ClientRequest{
			Path:   apiJobHTTPLog,
			Params: params,
		}
		clientResp *libhttp.ClientResponse
	)

	clientResp, err = cl.Client.Get(clientReq)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}

	var res = &libhttp.EndpointResponse{
		Data: &jlog,
	}
	err = json.Unmarshal(clientResp.Body, res)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}
	if res.Code == 200 {
		return jlog, nil
	}
	res.Data = nil
	return nil, res
}

// JobHTTPPause pause the HTTP job by its ID.
func (cl *Client) JobHTTPPause(id string) (jobHTTP *JobHTTP, err error) {
	var (
		logp   = `JobHTTPPause`
		params = url.Values{}
		header = http.Header{}
	)

	params.Set(`id`, id)

	var sign = Sign([]byte(params.Encode()), []byte(cl.opts.Secret))
	header.Set(HeaderNameXKarajoSign, sign)

	var (
		clientReq = libhttp.ClientRequest{
			Path:   apiJobHTTPPause,
			Header: header,
			Params: params,
		}
		clientResp *libhttp.ClientResponse
	)

	clientResp, err = cl.Client.Post(clientReq)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}

	var res = &libhttp.EndpointResponse{
		Data: &jobHTTP,
	}

	err = json.Unmarshal(clientResp.Body, res)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}
	if res.Code != 200 {
		res.Data = nil
		return nil, res
	}
	return jobHTTP, nil
}

// JobHTTPResume resume the HTTP job by its ID.
func (cl *Client) JobHTTPResume(id string) (jobHTTP *JobHTTP, err error) {
	var (
		logp   = `JobHTTPResume`
		params = url.Values{}
		header = http.Header{}
	)

	params.Set(`id`, id)

	var sign = Sign([]byte(params.Encode()), []byte(cl.opts.Secret))
	header.Set(HeaderNameXKarajoSign, sign)

	var (
		clientReq = libhttp.ClientRequest{
			Path:   apiJobHTTPResume,
			Header: header,
			Params: params,
		}
		clientResp *libhttp.ClientResponse
	)

	clientResp, err = cl.Client.Post(clientReq)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}

	var res = &libhttp.EndpointResponse{
		Data: &jobHTTP,
	}

	err = json.Unmarshal(clientResp.Body, res)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}
	if res.Code != 200 {
		res.Data = nil
		return nil, res
	}
	return jobHTTP, nil
}
