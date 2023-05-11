// SPDX-FileCopyrightText: 2022 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

let _env = {};
let _httpJobs = {};
let _jobs = {};

async function main() {
  runTimer();

  let wout = document.getElementById("out");
  let werr = document.getElementById("err");
  let delay = 10000;

  wout.innerHTML = "";
  werr.innerHTML = "";

  doRefresh();

  _refreshInterval = setInterval(() => {
    doRefresh();
  }, delay);
}

async function doRefresh() {
  let fres = await fetch("/karajo/api/environment");
  let res = await fres.json();
  if (res.code != 200) {
    werr.innerHTML = res.message;
    return;
  }

  _env = res.data;
  setTitle();

  renderJobs(_env.jobs);
  renderHttpJobs(_env.http_jobs);
}

function setTitle() {
  document.title = _env.name;
  document.getElementById("title").innerHTML = _env.name;
}

function runTimer() {
  let elTimer = document.getElementById("timer");

  setInterval(() => {
    elTimer.innerHTML = new Date().toUTCString();
  }, 1000);
}

async function jobInfo(jobID) {
  let job = _jobs[jobID];
  let el = document.getElementById(job._idInfo);
  let delay = 10000;

  if (el.style.display === "block") {
    el.style.display = "none";
    job._display = "none";
  } else {
    el.style.display = "block";
    job._display = "block";
  }
}

async function jobRunNow(jobID, jobPath) {
  let job = _jobs[jobID];
  let secret = document.getElementById("_secret").value;
  let epoch = parseInt(new Date().valueOf() / 1000);
  let req = {
    _karajo_epoch: epoch,
  };
  let body = JSON.stringify(req);

  let hash = CryptoJS.HmacSHA256(body, secret);
  let sign = hash.toString(CryptoJS.enc.Hex);
  let headers = {};

  switch (job.auth_kind) {
    case "github":
      headers["X-Hub-Signature-256"] = sign;
      break;
    case "hmac-sha256":
      headers[job.header_sign] = sign;
      break;
    // TODO: CryptoJS does not support ed25519 so we cannot support sourcehut auth right now.
  }

  let fres = await fetch(`/karajo/api/job/run${jobPath}`, {
    method: "POST",
    headers: headers,
    body: body,
  });

  let res = await fres.json();
  if (res.code !== 200) {
    console.error(res.message);
    return;
  }

  job = res.data;
  renderJobs([job]);
}

async function jobPause(id) {
  let secret = document.getElementById("_secret").value;
  let epoch = parseInt(new Date().valueOf() / 1000);
  let body = `_karajo_epoch=${epoch}&id=${id}`;

  let hash = CryptoJS.HmacSHA256(body, secret);
  let sign = hash.toString(CryptoJS.enc.Hex);

  let fres = await fetch("/karajo/api/job/pause", {
    method: "POST",
    headers: {
      "Content-Type": "application/x-www-form-urlencoded;charset=UTF-8",
      "x-karajo-sign": sign,
    },
    body: body,
  });

  let res = await fres.json();
  if (res.code !== 200) {
    console.error(res.message);
    return;
  }

  let job = res.data;
  renderJobs([job]);
}

async function jobResume(id) {
  let secret = document.getElementById("_secret").value;
  let epoch = parseInt(new Date().valueOf() / 1000);
  let body = `_karajo_epoch=${epoch}&id=${id}`;

  let hash = CryptoJS.HmacSHA256(body, secret);
  let sign = hash.toString(CryptoJS.enc.Hex);

  let fres = await fetch("/karajo/api/job/resume", {
    method: "POST",
    headers: {
      "Content-Type": "application/x-www-form-urlencoded;charset=UTF-8",
      "x-karajo-sign": sign,
    },
    body: body,
  });

  let res = await fres.json();
  if (res.code !== 200) {
    console.error(res.message);
    return;
  }

  let job = res.data;
  renderJobs([job]);
}

async function jobHttpInfo(jobID) {
  let job = _httpJobs[jobID];
  let el = document.getElementById(job._idInfo);
  let delay = 10000;

  if (el.style.display === "block") {
    el.style.display = "none";
    job._display = "none";

    if (job._logTimer != null) {
      clearInterval(job._logTimer);
      job._logTimer = null;
    }
  } else {
    el.style.display = "block";
    job._display = "block";

    jobHttpLogs(job);

    job._logTimer = setInterval(() => {
      jobHttpLogs(job);
    }, delay);
  }
}

async function jobHttpLogs(job) {
  let fres = await fetch("/karajo/api/job_http/logs?id=" + job.id);
  let res = await fres.json();
  if (res.code !== 200) {
    console.error(res.message);
    return;
  }

  job._log = res.data;
  renderJobHttpLog(job);
}

async function jobHttpPause(id) {
  let secret = document.getElementById("_secret").value;
  let epoch = parseInt(new Date().valueOf() / 1000);
  let q = `_karajo_epoch=${epoch}&id=${id}`;

  let hash = CryptoJS.HmacSHA256(q, secret);
  let sign = hash.toString(CryptoJS.enc.Hex);

  let fres = await fetch("/karajo/api/job_http/pause?" + q, {
    method: "POST",
    headers: {
      "x-karajo-sign": sign,
    },
  });
  let res = await fres.json();
  if (res.code !== 200) {
    console.error(res.message);
    return;
  }

  let job = _httpJobs[id];
  job = Object.assign(job, res.data);
  renderJobHttp(job);
}

async function jobHttpResume(id) {
  let secret = document.getElementById("_secret").value;
  let epoch = parseInt(new Date().valueOf() / 1000);
  let q = `_karajo_epoch=${epoch}&id=${id}`;

  let hash = CryptoJS.HmacSHA256(q, secret);
  let sign = hash.toString(CryptoJS.enc.Hex);

  let fres = await fetch("/karajo/api/job_http/resume?" + q, {
    method: "POST",
    headers: {
      "x-karajo-sign": sign,
    },
  });
  let res = await fres.json();
  if (res.code !== 200) {
    console.error(res.message);
    return;
  }

  let job = _httpJobs[id];
  job = Object.assign(job, res.data);
  renderJobHttp(job);
}

// renderJob render single job.
function renderJob(job) {
  renderJobAttributes(job);
  renderJobStatusRight(job);
  renderJobStatus(job);
}

function renderJobAttributes(job) {
  let el = document.getElementById(job._idAttrs);
  let out = ``;

  if (job.description) {
    out += `<div>${job.description}</div><br/>`;
  }

  out += `
    <div>ID: ${job.id}</div>
    <div>Path: ${job.path}</div>
    <div>Status: ${job.status || "-"}</div>
    <div>Last run: ${job.last_run}</div>
  `;

  if (job.schedule) {
    out += `
      <div>Schedule: ${job.schedule}</div>
      <div>Next run: ${job.next_run}</div>
    `;
  } else if (job.interval > 0) {
    out += `
      <div>Interval: ${job.interval / 1e9} seconds</div>
      <div>Next run: ${job.next_run}</div>
    `;
  }

  if (job.commands) {
    out += `
      <br/>
      <div class="job_commands">commands:
    `;
    job.commands.forEach(function (cmd, idx, list) {
      out += `<div> ${idx}: <tt>${cmd}</tt> </div>`;
    });
    out += `</div>`;
  }

  out += `
    <br/>
    <div>Log:
  `;

  if (job.logs == null) {
    job.logs = [];
  }

  job.logs.forEach(function (log, idx, list) {
    out += `<a
      href="/karajo/job/log?id=${job.id}&counter=${log.counter}"
      target="_blank"
      class="job-log ${log.status}"
    >
        #${log.counter}
    </a>`;
  });

  out += `&nbsp;<button onclick="jobRunNow('${job.id}', '${job.path}')">Run now</button>`;
  out += `&nbsp;<button onclick="jobPause('${job.id}')">Pause</button>`;
  out += `&nbsp;<button onclick="jobResume('${job.id}')">Resume</button>`;

  out += "</div>";

  el.innerHTML = out;
}

function renderJobStatusRight(job) {
  let elNextRun = document.getElementById(job._idStatusRight);

  let now = new Date();
  let nextRun = new Date(job.next_run);

  if (nextRun <= 0) {
    let lastRun = new Date(job.last_run);
    if (lastRun > 0) {
      elNextRun.innerText = `Last run ${lastRun.toUTCString()}`;
    }
    return;
  }

  let seconds = Math.floor((nextRun - now) / 1000);
  let hours = Math.floor(seconds / 3600);
  let minutes = Math.floor((seconds % 3600) / 60);
  let remSeconds = Math.floor(seconds % 60);
  elNextRun.innerText = `Next run ${hours}h  ${minutes}m ${remSeconds}s`;
}

function renderJobStatus(job) {
  let el = document.getElementById(job._idStatus);
  el.className = `name ${job.status}`;
}

// renderJobs render list of jobs.
function renderJobs(jobs) {
  let elJobs = document.getElementById("jobs");

  for (let name in jobs) {
    let job = jobs[name];

    job._id = `job_${job.id}`;
    job._idAttrs = `job_${job.id}_attrs`;
    job._idInfo = `job_${job.id}_info`;
    job._idStatusRight = `job_${job.id}_status_right`;
    job._idStatus = `job_${job.id}_status`;
    job._display = "none";

    if (_jobs != null) {
      let prevJob = _jobs[job.id];
      if (prevJob != null) {
        job._display = prevJob._display;
      }
    }

    _jobs[job.id] = job;

    let elJobInfo = document.getElementById(job._idInfo);
    if (elJobInfo != null) {
      renderJob(job);
      continue;
    }

    let out = `
      <div id="${job._id}" class="job">
        <div id="${job._idStatus}" class="name ${job.status}">
          <a href="#${job._id}" onclick='jobInfo("${job.id}")'>
            ${job.name}
          </a>
          <span id="${job._idStatusRight}" class="status_right"></span>
        </div>

        <div id="${job._idInfo}" style="display: ${job._display};">
          <div id="${job._idAttrs}" class="attrs"></div>
        </div>
      </div>
    `;

    let elJob = document.createElement("div");
    elJob.innerHTML = out;
    elJobs.appendChild(elJob);

    renderJob(job);
  }
}

function renderJobHttp(job) {
  renderJobHttpAttrs(job);
  renderJobHttpNextRun(job);
  renderJobHttpStatus(job);
}

function renderJobHttpAttrs(job) {
  let el = document.getElementById(job._idAttrs);
  let out = ``;

  if (job.description) {
    out += `<div>${job.description}</div><br/>`;
  }

  out += `
    <div>ID: ${job.id}</div>
    <div>HTTP URL: ${job.http_url}</div>
  `;

  if (job.http_headers) {
    out += `<div>HTTP headers: ${job.http_headers}</div>`;
  }

  out += `<div>HTTP timeout: ${job.http_timeout / 1e9}</div>`;

  if (job.interval) {
    out += `<div>Interval: ${job.interval / 1e9}s</div>`;
  }

  out += `
    <div>Last run: ${job.last_run}</div>
    <div>Next run: ${job.next_run}</div>
    <div>Status: ${job.status || ""}</div>
    <br/>
    <div class="actions">
  `;

  if (job.status == "paused") {
    out += `<button onclick="jobHttpResume('${job.id}')">Resume</button>`;
  } else {
    out += `<button onclick="jobHttpPause('${job.id}')">Pause</button>`;
  }

  out += `</div>`;
  el.innerHTML = out;
}

function renderJobHttpLog(job) {
  let el = document.getElementById(job._idLog);
  let out = "";

  for (let x = 0; x < job._log.length; x++) {
    out += `<p>${atob(job._log[x])}</p>`;
  }

  el.innerHTML = out;
  el.scrollTop = el.scrollHeight;
}

function renderJobHttpNextRun(job) {
  let elNextRun = document.getElementById(job._idStatusRight);

  let now = new Date();
  let nextRun = new Date(job.next_run);

  let seconds = Math.floor((nextRun - now) / 1000);
  if (seconds <= 0) {
    elNextRun.innerText = `Running ...`;
    return;
  }

  let hours = Math.floor(seconds / 3600);
  let minutes = Math.floor((seconds % 3600) / 60);
  let remSeconds = Math.floor(seconds % 60);
  elNextRun.innerText = `Next run in ${hours}h ${minutes}m ${remSeconds}s`;
}

function renderJobHttpStatus(job) {
  let el = document.getElementById(job._idStatus);
  el.className = `name ${job.status}`;
}

function renderHttpJobs(httpJobs) {
  let out = "";
  let elHttpJobs = document.getElementById("http_jobs");

  for (let name in httpJobs) {
    let httpJob = httpJobs[name];

    httpJob._id = `jobhttp_${httpJob.id}`;
    httpJob._idAttrs = `jobhttp_${httpJob.id}_attrs`;
    httpJob._idInfo = `jobhttp_${httpJob.id}_info`;
    httpJob._idLog = `jobhttp_${httpJob.id}_log`;
    httpJob._idStatus = `jobhttp_${httpJob.id}_status`;
    httpJob._idStatusRight = `jobhttp_${httpJob.id}_status_right`;
    httpJob._display = "none";
    httpJob._logTimer = null;

    if (_httpJobs != null) {
      let prevJob = _httpJobs[httpJob.id];
      if (prevJob != null) {
        httpJob._display = prevJob._display;
        httpJob._logTimer = prevJob._logTimer;
      }
    }

    _httpJobs[httpJob.id] = httpJob;

    let elJob = document.getElementById(httpJob._id);
    if (elJob != null) {
      renderJobHttp(httpJob);
      continue;
    }

    out = `
      <div id="${httpJob._id}" class="jobhttp">
        <div id="${httpJob._idStatus}" class="name ${httpJob.status}">
          <a href="#${httpJob._id}" onclick='jobHttpInfo("${httpJob.id}")'>
            ${httpJob.name}
          </a>
          <span id="${httpJob._idStatusRight}" class="status_right"></span>
        </div>

        <div id="${httpJob._idInfo}" style="display: ${httpJob._display};">
          <div id="${httpJob._idAttrs}" class="attrs"></div>
          <div id="${httpJob._idLog}" class="log"></div>
        </div>
      </div>
    `;

    elHttpJobs.innerHTML += out;
    renderJobHttp(httpJob);
  }
}
