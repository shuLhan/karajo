# karajo

Module karajo implement HTTP workers and manager similar to AppEngine
cron.

karajo has the web user interface (WUI) for monitoring the jobs that run on
port 31937 by default and can be configurable.

A single instance of karajo is configured through an Environment or ini file
format.
There are three configuration sections: one to configure the server, one to
configure the logs, and another one to configure one or more jobs to be
executed.


## Configuration file format

This section describe the file format when loading karajo environment from
file.

### karajo server

This section has the following format,

```
[karajo]
name = <string>
listen_address = [<ip>:<port>]
http_timeout = [<duration>]
dir_logs = <path>
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

The "dir_logs" option define the path to directory where each log from job
will be stored.
If this value is empty, all job logs will be written to stdout and stderr.

By default, each job has its own log file using the job name and ".log" as
suffix in the filename.


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

## License

```
Copyright 2021, M. Shulhan (ms@kilabit.info).
All rights reserved.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are met:

1. Redistributions of source code must retain the above copyright notice, this
   list of conditions and the following disclaimer.

2. Redistributions in binary form must reproduce the above copyright notice,
   this list of conditions and the following disclaimer in the documentation
   and/or other materials provided with the distribution.

3. Neither the name of copyright holder nor the names of its contributors may
   be used to endorse or promote products derived from this software without
   specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDERS OR CONTRIBUTORS BE LIABLE
FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR
SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER
CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY,
OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
```
