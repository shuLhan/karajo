# karajo

Module karajo implement HTTP workers and manager, similar to cron but works
only on HTTP.

karajo has the web user interface (WUI) for monitoring the jobs that run on
URL http://127.0.0.1:31937/karajo by default and can be configurable.

A single instance of karajo is configured through code or a configuration file
using ini file format.

Features,

* Running job on specific interval
* Preserve the job states on restart
* Able to pause and resume specific job
* HTTP APIs to programmatically interact with karajo

## Configuration file format

This section describe the file format when loading karajo environment from
file.

There are three configuration sections: one to configure the server, one to
configure the logs, and another one to configure one or more jobs to be
executed.

### karajo server

This section has the following format,

```
[karajo]
name = <string>
listen_address = [<ip>:<port>]
http_timeout = [<duration>]
dir_base = <path>
```

The "name" option define the name of the service.
It will be used for title on the web user interface, as log prefix, for file
prefix on the jobs state, and as file prefix on log files.
If this value is empty, it will be set to "karajo".

The "listen_address" define the address for WUI, default to ":31937".

The "http_timeout" define the HTTP timeout when executing the job, default to
5 minutes.
The value of this option is using the Go time.Duration format, for example,
30s for 30 seconds, 1m for 1 minute.

The "dir_base" option define the base directory where configuration, job
state, and log stored.
This field is optional, default to current directory.
The structure of directory follow the UNIX system,

----
$DirBase
|
+-- /etc/karajo/karajo.conf
|
+-- /var/log/karajo/job/$Job.ID
|
+-- /var/run/karajo/job/$Job.ID
----

Each job log stored under directory /var/log/karajo/job and the job state
under directory /var/run/karajo/job.


### karajo job

This section has the following format,

```
[karajo "job"]
name = <string>
description = <string>
http_url = <URL>
http_header = <string ":" string>
http_insecure = <bool>
interval = <duration>
```

The "name" option define the job name.
Each job must have unique name or only the first one will be processed.

The "description" field define the job description.
It could be plain text or simple HTML.

The "http_url" define the HTTP URL where the job will be executed.
This field is required.

The "http_header" option define optional HTTP headers that will send when
executing the "http_url".
This option can be declared more than one.

The "http_insecure" option can be set to true if the "http_url" is HTTPS with
unknown certificate authority.

The "interval" option define the duration when job will be repeatedly
executed.
This field is required, if not set or invalid it will set to 30 seconds.
If one have job that need to run less than 30 seconds, it should be run on
single program.


## HTTP APIs

The karajo service is a HTTP server.
Its provide HTTP APIs to interact with the system.
The following sub-sections describe each HTTP APIs request and response.

All HTTP response is encoded in the JSON format, with the following wrapper,

```
{
	"code": <number>,
	"message": <string>,
	"data": <array|object>
}
```

* `code`: the response code, equal to HTTP status code.
* `message`: the error message that describe why request is fail.
* `data`: the dynamic data, specific to each endpoint.

### Schemas

Job schema,

```
{
	"ID": <string>,
	"Name": <string>,
	"Description": <string>,
	"HttpUrl": <string>,
	"HttpHeaders": [<string>],
	"HttpInsecure": <boolean>,
	"HttpTimeout": <number>,
	"Interval": <number>,
	"MaxRequest": <number>,
	"NumRequests": <number>,
	"LastRun": <string>,
	"NextRun": <string>,
	"Status": <string>,
}
```

* `ID`: unique job ID
* `Name`: human representation of job name.
* `Description`: job description, can be HTML.
* `HttpUrl`: the URL where job will be executed.
* `HttpHeaders`: list of string, in the format of HTTP header "Key: Value",
  which will be send when invoking the job at `HttpUrl`.
* `HttpTimeout`: number of nano-seconds when the job will be considered to be
  timeout.
* `Interval`: a period of nano-seconds when the job will be executed.
* `MaxRequest`: maximum number of job can be requested at a time.
* `NumRequests`: current number of job running.
* `LastRun`: date and time when the job last run, in the format RFC3339,
* `NextRun`: date and time when the next job will be executed, in the format
  RFC3339.
* `Status`: status of the last job running, its either "started, "success",
  "failed", or "paused".

### Get environment

Get the current karajo environment.

**Request**

```
GET /karajo/api/environment
```

**Response**

On success, it will return the Environment object,

```
{
	"Name": <string>,
	"ListenAddress": <string>,
	"HttpTimeout": <number>
	"DirLogs": <string>,
	"Jobs": [<Job>]
}
```

* `Name`: the karajo server name.
* `ListenAddress`: the address where karajo HTTP server listening for request.
* `HttpTimeout`: default HTTP timeout for job in nano-second.
* `DirLogs`: the path to directory where the each job logs will be stored.
* `Jobs`: list of Job.


### Get job detail

HTTP API to get specific job information by its ID.

**Request**

```
GET /karajo/api/job?id=<string>
```

Parameters,

* `id`: the job ID.

**Response**

On success, it will return the Job schema.

On fail, it will return

* `400`: for invalid or empty job ID


### Get job logs

Get the last logs from specific job by its ID.

**Request**

```
GET /karajo/api/job/logs?id=<string>
```

Parameters,

* `id`: the job ID.

**Response**

On success it will return list of string, contains log execution and the
response from executing the `HttpUrl`.

On fail, it will return

* `400`: invalid or empty job ID.


### Pause the job

Pause the job execution by its ID.

**Request**

```
POST /karajo/api/job/pause/<id>
```

Parameters,

* `id`: the job ID.

**Response**

On success it will return the Job schema with field `Status` set to `paused`.

On fail it will return

* `400`: invalid or empty job ID.


### Resume the job

HTTP API to resume paused job by its ID.

**Request**

```
POST /karajo/api/job/resume/<id>
```

Parameters,

* `id`: the job ID.

**Response**

On success it will return the Job schema related to the ID with field
`Status` reset back to `started`.


## Example

Given the following karajo configuration file named `karajo_test.conf` with
content as

```
[karajo]
name = My worker
listen_address = 127.0.0.1:31937
http_timeout = 5m0s
dir_base = testdata

[karajo "job"]
name = Test fail
description = The job to test what the user interface and logs look likes \
	if its <b>fail</b>.
http_url = http://127.0.0.1:31937/karajo/test/job/fail
http_header = A: B
http_header = C: D
http_insecure = false
interval = 20s
max_requests = 2

[karajo "job"]
name = Test success
description = The job to test what the user interface and logs look likes \
	if its <i>success</i>.
http_url = /karajo/test/job/success
http_header = X: Y
http_insecure = false
interval = 20s
max_requests = 1
```

Run the `karajo` program,

```
$ karajo -config karajo_test.conf
```

And then open http://127.0.0.1:31937/karajo in your web browser to see the job
status and logs.


## License

Copyright 2021, M. Shulhan (ms@kilabit.info).

This program is free software: you can redistribute it and/or modify it under
the terms of the GNU General Public License as published by the Free Software
Foundation, either version 3 of the License, or (at your option) any later
version.

This program is distributed in the hope that it will be useful, but WITHOUT
ANY WARRANTY;
without even the implied warranty of MERCHANTABILITY or FITNESS FOR A
PARTICULAR PURPOSE.
See the GNU General Public License for more details.

You should have received a copy of the GNU General Public License along with
this program.  If not, see <http://www.gnu.org/licenses/>.
