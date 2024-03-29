# karajo

## Overview

Module karajo implement HTTP workers and manager, similar to cron but works
and manageable with HTTP.
A karajo server manage one or more jobs.
A job is function or list of commands that executed once its triggered either
by timer or from external HTTP request.

Karajo has the web user interface (WUI) for monitoring the jobs that can be
accessed at `http://127.0.0.1:31937/karajo` by default and can be configurable.

A single instance of karajo can be configured through code or from file using
INI file format.

Features,

* Running job on specific interval.
* Running job on specific schedule.
* Trigger HTTP request to external server on specific interval.
* Preserve the job states on restart.
* Able to pause and resume specific job.
* Trigger job using HTTP request (webhook). Supported webhook are `github`,
  `sourcehut`, or custom `hmac-sha256` (default).
* HTTP APIs to programmatically interact with server
* User authentication

Workflow on karajo,

```
                  karajo
                /-----------------------------\
                |                             |
                |   +---+         +---+       |
                |   |   | timer   |   | timer |
                |   |   v         |   v       |
                | +---------+   +-------+     |
  INTERNET <----- | JobHttp |   | Job   | <----- INTERNET
                | +---------+   +-------+     |
                \------------------|----------/
                                   |
                                   v
                        +-----------------+
                        | Commands / Call |
                        +-----------------+
```


## Configuration

This section describe the file format when loading karajo environment from
file.

There are three configuration sections: one to configure the server, one to
configure the internal Job, and another one to configure the external Job HTTP
to be executed.


###  Environment (the server)

The global environment section has the following format,

```
[karajo]
name = <string>
listen_address = [<ip>:<port>]
http_timeout = [<duration>]
dir_base = <path>
dir_public = <path>
secret = <string>
max_job_running = <number>
```

`name`:: Name of the service.
The Name will be used for title on the web user interface, as log
prefix, as file prefix on the jobs state, and as file prefix on
log files.
If this value is empty, it will be set to "karajo".

`listen_address`:: Define the address for WUI, default to ":31937".

`dir_base`:: Define the base directory where configurations, job's state, and
job's log stored.

This field is optional, default to current directory.
The structure of directory follow the common UNIX system,

```
$dir_base
|
+-- /etc/karajo/ +-- karajo.conf
|                +-- job.d/
|                +-- job_http.d/
|                +-- user.conf
|
+-- /var/lib/karajo/job/$Job.ID
|
+-- /var/log/karajo/ +-- job/$Job.ID
|                    +-- job_http/$Job.ID
|
+-- /var/run/karajo/job/$Job.ID
```

Each job log stored under directory /var/log/karajo/job and the job state
under directory /var/run/karajo/job.

`dir_public`:: Define a path to serve to the public.
While the WUI is served under "/karajo", a directory dir_public will be served
under "/".
A dir_public can contains sub directory as long as its name is not "karajo".

`secret`:: Define the default secret to authorize the incoming HTTP request.
The signature is generated from HTTP payload (query or body) with
HMAC+SHA-256.
The signature is read from HTTP header "X-Karajo-Sign" as hex
string.
This field is optional, if its empty the new secret will be
generated and printed to standard output on each run.

`http_timeout`:: Define the global HTTP client timeout when executing
each jobs.
This field is optional, default to 5 minutes.
The value of this option is using the Go
[time.Duration](https://pkg.go.dev/time#Duration)
format, for example, "30s" for 30 seconds, "1m" for 1 minute.

`max_job_running`:: Define the global maximum job running at the same time.
This field is optional default to 1.

### Notification

Karajo server support sending notification when the job success or failed
with inline log.

#### Email

The following email configuration is defined in the same file as Environment,

```
[notif "$name"]
kind = email
smtp_server =
smtp_user =
smtp_password =
smtp_insecure =
to = your@mailbox.com
to = ...
```

`$name`:: unique name for notification.

`kind`:: notification type, must be set to "email".

`smtp_server`:: The SMTP server, using the following URL format,

```
[ scheme "://" ](domain | IP-address)[":" port]
```

where scheme is "smtps" or "smtp+starttls".
For smtps, if no port is given, client will connect to server at port 465.
For smtp+starttls, if no port is given, client will connect to server at
port 587.

`smtp_user`:: user name for submission.
If start with '$' it will read from system environment.

`smtp_password`:: password for smtp_user submission.
If start with '$' it will read from system environment.

`smtp_insecure`::  if set to true it will disable verifying remote
certificate when connecting with TLS or STARTTLS.

`from`:: the from address that send the email.

`to`:: email address that will receive notification, can be defined more
than one.

###  User

The Karajo WUI can be secured with login, where user must authenticated
using name and password before they can view the dashboard.
By default the user list is empty, which allow public to view the dashboard.

The user account can be set using configuration or by code.

The configuration file for user is in `$dir_base/etc/karajo/user.conf`,
using the following format,

```
[user "$name"]
password = <$bcrypt_hash>
```

Each user $name is unique.
The `$bcrypt_hash` is the password of user, stored as hash using bcrypt
version 2a (`$2a$`).

In the code, one can register the same things using field `Users` in the
`Environment`,

```
env := karajo.Environment{
	Users: map[string]*karajo.User{
		`yourname`: &User{
			// Hash of password `s3cret` using bcrypt v2a.
			Password: `$2a$10$9XMRfqpnzY2421fwYm5dd.CidJf7dHHWIESeeNGXuajHRf.Lqzy7a`
		},
	}
}
```


###  Job

Job is the worker that run a function or list of commands triggered from
external HTTP request or by timer.

A job configuration can be defined along with main configuration,
`karajo.conf` or split into separate files inside the
`$dir_base/etc/karajo/job.d/`, with suffix `.conf`.
The Job configuration have the following format,

```
[job "name"]
description = <string>
schedule = <string>
interval = <duration>
path = <string>
auth_kind = <string>
header_sign = <string>
secret = <string>
log_retention = <number>
command = <string>
...
command = <string>
notif_on_success = <string>
...
notif_on_failed = <string>
...
```

`name`:: Define the job name.
The job name is used for logging, normalized to ID.
This field is required and should unique between Job.

`description`:: The description of the Job.
It could be plain text or simple HTML.


`schedule`:: A timer that run periodically based on calendar or day time.

A schedule is divided into monthly, weekly, daily, hourly, and minutely.
A date and time in schedule is in UTC.
Example of schedules,

* monthly@1,15@18:00 = on day 1 and 15 every month at 6 PM UTC.
* weekly@Sunday,Tuesday,Friday@15:00 = every Sunday, Tuesday, and Friday on
  each week at 3 PM UTC.
* daily@00:00,06:00,12:00,18:00 = every day at midnight, 6 AM, and 12 PM UTC.
* hourly@0,15,30,45 = on minutes 0, 15, 30, 45 every hour.

See
[time.Schedule](https://pkg.go.dev/git.sr.ht/~shulhan/pakakeh.go/lib/time#Scheduler)
for format of schedule.
If both Schedule and Interval set, only Schedule will be processed.


`interval`:: Define the duration when job will be repeatedly executed.
This field is optional, if not set the Job can only run when receiving HTTP
request.
If both Schedule and Interval set, only Schedule will be processed.

`path`:: HTTP path where Job can be triggered using HTTP.
The `path` is automatically prefixed with "/karajo/api/job_exec/run", it is
not static.
For example, if it set to "/my", then the actual path would be
"/karajo/api/job_exec/run/my".
This field is optional and must unique between Job.

`auth_kind`:: Define the kind of authorization to trigger Job.
Supported AuthKind are

* `github`: the signature read from "X-Hub-Signature-256" and
  compare it by signing request body with Secret using HMAC-SHA256.
  If the header is empty, it will check another header "X-Hub-Signature" and
  then sign the request body with Secret using HMAC-SHA1.

* `hmac-sha256` (default): the signature read from HeaderSign and compare it
  by signing request body with Secret using HMAC-SHA256.

* `sourcehut`: See [man.sr.ht](https://man.sr.ht/api-conventions.md#webhooks)

`header_sign`:: Define custom HTTP header where the signature is read.
Default to "X-Karajo-Sign" if its empty.

`secret`:: Define a string to validate the signature of request based on the
AuthKind.
If its empty, it will be set to global Secret from Environment.

`log_retention`:: Define the maximum number of logs to keep in storage.
This field is optional, default to 5.

`command`:: List of command to be executed.

This option can be defined multiple times.
It contains command to be executed, in order from top to bottom.
The following environment variables are available inside the command:

`notif_on_success`:: List of notification that will be triggered when job
finish with status "success".
This option can be defined multiple times.

`notif_on_failed`:: List of notification that will be triggered when job
finish with status "failed".
This option can be defined multiple times.


### JobHttp

A JobHttp is a periodic job that send HTTP request to external HTTP server
(or to karajo Job itself).

A JobHttp configuration can be defined along with the main configuration,
`karajo.conf` or split into separate files inside the
`$dir_base/etc/karajo/job_http.d/`, with suffix `.conf`.

Each JobHttp has the following configuration,

```
[job.http "name"]
description = <string>
secret = <string>
header_sign = <string>
schedule = <string>
interval = <duration>

http_method = [GET|POST|PUT|DELETE]
http_url = <URL>
http_request_type = [query|form|json]
http_header = <string ":" string>
http_timeout = <duration>
http_insecure = <bool>

notif_on_success = <string>
...
notif_on_failed = <string>
...
```

`name`:: The job name.
Each job must have unique name.
If two or more jobs have the same name only the first one will be processed.

`description`:: The job description.
It could be plain text or simple HTML.

`secret`:: Define a string to sign the request payload with HMAC+SHA-256.
The signature is sent on HTTP header "X-Karajo-Sign" as hex string.
If its empty, it will be set to global Secret from Environment.


`header_sign`:: Define the HTTP header where the signature will be written in
request.
Default to "X-Karajo-Sign" if its empty.


`schedule`:: A timer that run periodically based on calendar or day time.

A schedule is divided into monthly, weekly, daily, hourly, and minutely.
A date and time in schedule is in UTC.
Example of schedules,

* monthly@1,15@18:00 = on day 1 and 15 every month at 6 PM UTC.
* weekly@Sunday,Tuesday,Friday@15:00 = every Sunday, Tuesday, and Friday on
  each week at 3 PM UTC.
* daily@00:00,06:00,12:00,18:00 = every day at midnight, 6 AM, and 12 PM UTC.
* hourly@0,15,30,45 = on minutes 0, 15, 30, 45 every hour.

See
[time.Schedule](https://pkg.go.dev/git.sr.ht/~shulhan/pakakeh.go/lib/time#Scheduler)
for format of schedule.
If both Schedule and Interval set, only Schedule will be processed.


`interval`:: Define the interval when job will be executed.
This field is required, if not set or invalid it will set to 30 seconds.
If one have job that need to run less than 30 seconds, it should be run on
single program.
If both Schedule and Interval set, only Schedule will be processed.

`http_method`:: Define the HTTP method to be used in request for job
execution.
Its accept only GET, POST, PUT, or DELETE.
This field is optional, default to GET.

`http_url`:: Define the HTTP URL where the job will be executed.
This field is required.

`http_request_type`:: Define the header Content-Type to be set on
request.

Its accept,

* (empty string): no header Content-Type to be set.
* query: no header Content-Type to be set, reserved for future use.
* form: header Content-Type set to "application/x-www-form-urlencoded",
* json: header Content-Type set to "application/json".

The type "form" and "json" only applicable if the http_method is POST or PUT.
This field is optional, default to query.

Each Job execution send the parameter named `_karajo_epoch` with value is
current server Unix time.
If the request type is `query` then the parameter is inside the query URL.
If the request type is `form` then the parameter is inside the body.
If the request type is `json` then the parameter is inside the body as JSON
object, for example `{"_karajo_epoch":1656750073}`.

`http_header`:: Define optional HTTP headers that will send when executing the
job.
This option can be declared more than one.

`http_timeout`:: Define the HTTP timeout when executing the job.

If its zero, it will set from the Environment.HttpTimeout.
To make job run without timeout, set the value to negative.
The value of this option is using the Go time.Duration format, for example,
30s for 30 seconds, 1m for 1 minute, 1h for 1 hour.

`http_insecure`:: Can be set to true if the "http_url" is HTTPS with unknown
Certificate Authority.

`notif_on_success`:: List of notification that will be triggered when job
finish with status "success".
This option can be defined multiple times.

`notif_on_failed`:: List of notification that will be triggered when job
finish with status "failed".
This option can be defined multiple times.


## Examples

This section show some examples of creating Job and JobHttp using
configuration and code.
You can run the example from this repository by executing,

    $ go run ./internal/cmd/karajo-example

For Job as configuration, we can put it along main configuration
`$dir_base/etc/karajo/karajo.conf` or split into file inside
`$dir_base/etc/karajo/job.d/`.

For JobHttp as configuration, we can put it along main configuration
`$dir_base/etc/karajo/karajo.conf` or split into file inside
`$dir_base/etc/karajo/job_http.d/`.

For Job or JobHttp as code, the main program would looks like these,

```
package main

import (
	"log"

	"git.sr.ht/~shulhan/karajo"
)

func main() {
	var (
		env = karajo.NewEnvironment()

		k   *karajo.Karajo
		err error
	)

	env.DirBase = `testdata` // For example only.
	env.Secret = `s3cret` // For example only.

	// Add one or more job here...

	k, err = karajo.New(env)
	if err != nil {
		log.Fatal(err)
	}

	err = k.Start()
	if err != nil {
		log.Fatal(err)
	}
}
```


### Job with interval

The following job will run command `echo "Hello world from interval"` every 1
minute,

```
[job "interval-1m"]
description = Job with interval 1 minute.
interval = 1m
path = /interval-1m
command = echo "Hello world from interval"
```

The same job can be create using the following code,

```
...
	env.Jobs[`interval-1m-code`] = &karajo.Job{
		JobBase: karajo.JobBase{
			Description: `Job with interval 1 minute, initialize by code.`,
			Interval:    1 * time.Minute,
		},
		Path: `/interval-1m-code`,
		Call: helloWorldFromInterval,
	}
...
func helloWorldFromInterval(log io.Writer, epr *libhttp.EndpointRequest) error {
	fmt.Fprintln(log, `Hello world from interval with code`)
	return nil
}
```

Since both of these job has the `Path` set, we can be trigger it using HTTP.
First we need a payload, and from that payload we generate the signature using
the secret,

```
$ PAYLOAD='{"_karajo_epoch":1677350854}'
$ echo -n $PAYLOAD | openssl dgst -sha256 -hmac "s3cret" -hex
SHA2-256(stdin)=fef16cabebdcbdcc7fdbfb9bd5a01c00803af7568a05054e87b2239f84f38c54
```

The `fef16cabe...` is the keyed hash signature that we will send to
authenticate the request.

Then we send the `$PAYLOAD` inside the body with signature inside the header,

```
$ curl \
	--header "X-Karajo-Sign:fef16cabebdcbdcc7fdbfb9bd5a01c00803af7568a05054e87b2239f84f38c54" \
	--json '{"_karajo_epoch":1677350854}' \
	http://127.0.0.1:31937/karajo/api/job_exec/run/hello-world
{"code":200,"message":"OK","data":{"Logs":[{"JobID":"hello-world","Name":"hello-world.27.success","Status":"success",...
```

(Change the path to /karajo/api/job_exec/run/hello-world-code to trigger the
code one)


### Job with schedule

The following job will run command `echo "Hello world from schedule"` in
minutes of 0, 5, 10, 15, 20, 25, 30, 35, 40, 45, 50, 55 every hour.

```
[job "schedule-hourly-5m"]
description = Job with schedule every hour at minutes 5.
path = /schedule-hourly-5m
secret = s3cret
schedule = hourly@0,5,10,15,20,25,30,35,40,45,50,55
command = echo "Hello world from schedule"
```

The same job can be created using the following code,

```
...
	env.Jobs[`schedule-hourly-5m-code`] = &karajo.Job{
		JobBase: karajo.JobBase{
			Description: `Job with schedule every hour at minutes 5.`,
			Schedule:    `hourly@0,5,10,15,20,25,30,35,40,45,50,55`,
		},
		Path: `/schedule-hourly-5m-code-code`,
		Call: helloWorldFromSchedule,
	}
...
func helloWorldFromSchedule(log io.Writer, epr *libhttp.EndpointRequest) error {
	fmt.Fprintln(log, `Hello world from schedule`)
	return nil
}
...
```


### Job as webhook

Job as webhook can only triggered by sending HTTP request to Job's path.
To create a webhook, do not set the interval and schedule fields.
The following example show how to create webhook for Github,

```
[job "webhook-github"]
description = Webhook using github authentication.
auth_kind = github
secret = s3cret
path = /webhook-github
command = echo "Webhook using github authentication"
```

The same configuration can be create using code below,

```
...
	env.Jobs[`webhook-github-code`] = &karajo.Job{
		JobBase: karajo.JobBase{
			Description: `Webhook using github authentication`,
		},
		AuthKind: `github`,
		Secret:   `s3cret`,
		Path:     `/webhook-github-code`,
		Call:     webhookWithGithub,
	}
...
func webhookWithGithub(log io.Writer, epr *libhttp.EndpointRequest) error {
	fmt.Fprintln(log, `Hello world from Webhook github`)
	return nil
}
```

Once the server running, you can register the following webhook path to
Github,

* `https://<YOUR_KARAJO_IP>/karajo/api/job_exec/run/webhook-github`, or
* `https://<YOUR_KARAJO_IP/karajo/api/job_exec/run/webhook-github-code`

using the secret: `s3cret`.


### JobHttp by interval

The concept of interval in JobHttp is similar with Job, but instead of running
a command it send HTTP request to external HTTP server every N interval.

The following configuration create JobHttp that trigger the POST request to
webhook-github Job that we create earlier every 90 seconds,

```
[job.http "interval-90s"]
description = Trigger our webhook-github every 90 seconds.
secret = s3cret
header_sign = X-Hub-Signature-256
interval = 90s
http_method = POST
http_url = /karajo/api/job_exec/run/webhook-github
http_request_type = json
```

The same configuration can be created using code as below,

```
...
	env.HttpJobs[`interval-90s-code`] = &karajo.JobHttp{
		JobBase: karajo.JobBase{
			Description: `Trigger our webhook-github every 90 seconds by code.`,
			Interval:    90 * time.Second,
		},
		Secret:          `s3cret`,
		HeaderSign:      `X-Hub-Signature-256`,
		HttpMethod:      `POST`,
		HttpUrl:         `/karajo/api/job_exec/run/webhook-github`,
		HttpRequestType: `json`,
	}
...
```


### JobHttp by schedule

The following configuration create JobHttp that trigger the POST request to
webhook-github-code that we create earlier on minutes 0, 6, 12, 18, 24, 30,
36, 42, 48, 54 of every hour.

```
[job.http "schedule-hourly-6m"]
description = Trigger our webhook-github-code by schedule every 6m.
secret = s3cret
header_sign = X-Hub-Signature-256
schedule = hourly@0,6,12,18,24,30,36,42,48,54
http_method = POST
http_url = /karajo/api/job_exec/run/webhook-github-code
http_request_type = json
```

The same configuration can be created using code as below,

```
...
	env.HttpJobs[`schedule-6m-code`] = &karajo.JobHttp{
		JobBase: karajo.JobBase{
			Description: `Trigger our webhook-github-code by schedule every 6m.`,
			Schedule:    `hourly@0,6,12,18,24,30,36,42,48,54`,
		},
		Secret:          `s3cret`,
		HeaderSign:      `X-Hub-Signature-256`,
		HttpMethod:      `POST`,
		HttpUrl:         `/karajo/api/job_exec/run/webhook-github-code`,
		HttpRequestType: `json`,
	}
...
```


## Development

[CHANGELOG](CHANGELOG.html) - History of each releases.

[HTTP APIs](http_api.html) - The exposed HTTP API documentation for karajo
server.

[Repository](https://git.sr.ht/~shulhan/karajo) - The source code repository.

[Mailing list](https://lists.sr.ht/~shulhan/karajo) - Place for discussion and
sending patches.

[Issues](https://todo.sr.ht/~shulhan/karajo) - Link to open an issue or
request for new feature.


## License

Copyright 2021-2023, M. Shulhan (ms@kilabit.info).

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
this program.  If not, see <https://www.gnu.org/licenses/>.
