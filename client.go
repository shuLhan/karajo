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

// Env get the server environment.
func (cl *Client) Env() (env *Env, err error) {
	var (
		logp = `Env`

		res     *libhttp.EndpointResponse
		resBody []byte
	)

	_, resBody, err = cl.Client.Get(apiEnv, nil, nil)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}

	env = &Env{}
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

// JobPause pause the JobExec by its ID.
func (cl *Client) JobPause(id string) (job *JobExec, err error) {
	var (
		logp   = `JobPause`
		now    = TimeNow().UTC().Unix()
		params = url.Values{}
		header = http.Header{}

		res     *libhttp.EndpointResponse
		body    string
		sign    string
		resBody []byte
	)

	params.Set(paramNameKarajoEpoch, strconv.FormatInt(now, 10))
	params.Set(paramNameID, id)

	body = params.Encode()

	sign = Sign([]byte(body), []byte(cl.opts.Secret))
	header.Set(HeaderNameXKarajoSign, sign)

	_, resBody, err = cl.Client.PostForm(apiJobPause, header, params)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}

	job = &JobExec{}
	res = &libhttp.EndpointResponse{
		Data: job,
	}

	err = json.Unmarshal(resBody, &res)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}
	if res.Code != http.StatusOK {
		res.Data = nil
		return nil, res
	}

	return job, nil
}

// JobResume resume the JobExec execution by its ID.
func (cl *Client) JobResume(id string) (job *JobExec, err error) {
	var (
		logp   = `JobResume`
		now    = TimeNow().UTC().Unix()
		params = url.Values{}
		header = http.Header{}

		res     *libhttp.EndpointResponse
		body    string
		sign    string
		resBody []byte
	)

	params.Set(paramNameKarajoEpoch, strconv.FormatInt(now, 10))
	params.Set(paramNameID, id)

	body = params.Encode()

	sign = Sign([]byte(body), []byte(cl.opts.Secret))
	header.Set(HeaderNameXKarajoSign, sign)

	_, resBody, err = cl.Client.PostForm(apiJobResume, header, params)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}

	job = &JobExec{}
	res = &libhttp.EndpointResponse{
		Data: job,
	}

	err = json.Unmarshal(resBody, &res)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}
	if res.Code != http.StatusOK {
		res.Data = nil
		return nil, res
	}

	return job, nil
}

// JobRun trigger the JobExec by its path.
func (cl *Client) JobRun(jobPath string) (job *JobExec, err error) {
	var (
		logp       = `JobExec`
		timeNow    = TimeNow()
		apiJobPath = path.Join(apiJobRun, jobPath)
		header     = http.Header{}

		req = JobHttpRequest{
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

	httpRes, resBody, err = cl.Client.PostJSON(apiJobPath, header, &req)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}
	if httpRes.StatusCode == http.StatusNotFound {
		return nil, errJobNotFound(jobPath)
	}

	res = &libhttp.EndpointResponse{
		Data: &job,
	}
	err = json.Unmarshal(resBody, res)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}
	if res.Code >= 400 {
		return nil, res
	}
	return job, nil
}

// JobLog get the JobExec log by its ID and counter.
func (cl *Client) JobLog(jobID string, counter int) (joblog *JobLog, err error) {
	var (
		logp   = `JobLog`
		params = url.Values{}

		res     *libhttp.EndpointResponse
		resBody []byte
	)

	params.Set(paramNameID, jobID)
	params.Set(paramNameCounter, strconv.Itoa(counter))

	_, resBody, err = cl.Client.Get(apiJobLog, nil, params)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}

	res = &libhttp.EndpointResponse{
		Data: &joblog,
	}
	err = json.Unmarshal(resBody, res)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}
	if res.Code == 200 {
		return joblog, nil
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

	_, resBody, err = cl.Client.Get(apiJobHttp, nil, params)
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

// JobHttpLog get the job logs by its ID.
func (cl *Client) JobHttpLog(id string, counter int) (jlog *JobLog, err error) {
	var (
		logp   = `JobHttpLog`
		params = url.Values{}

		res     *libhttp.EndpointResponse
		resBody []byte
	)

	params.Set(paramNameID, id)
	params.Set(paramNameCounter, strconv.Itoa(counter))

	_, resBody, err = cl.Client.Get(apiJobHttpLog, nil, params)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}

	res = &libhttp.EndpointResponse{
		Data: &jlog,
	}
	err = json.Unmarshal(resBody, res)
	if err != nil {
		return nil, fmt.Errorf(`%s: %w`, logp, err)
	}
	if res.Code == 200 {
		return jlog, nil
	}
	res.Data = nil
	return nil, res
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

	_, resBody, err = cl.Client.Post(apiJobHttpPause, header, params)
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

	_, resBody, err = cl.Client.Post(apiJobHttpResume, header, params)
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
