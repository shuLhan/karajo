// SPDX-License-Identifier: GPL-3.0-or-later
// SPDX-FileCopyrightText: 2022 M. Shulhan <ms@kilabit.info>

let _env = {};
let _httpJobs = {};
let _jobs = {};

async function main() {
  runTimer();

  let wout = document.getElementById("out");
  wout.innerHTML = "";
  let werr = document.getElementById("err");
  werr.innerHTML = "";

  doRefresh();
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
  document.getElementById("version").innerText = _env.version;

  renderJobs(_env.jobs);
  renderHttpJobs(_env.http_jobs);

  handleFragment(_env.jobs, _env.http_jobs);
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

async function jobCancel(id) {
  let secret = document.getElementById("_secret").value;
  let epoch = parseInt(new Date().valueOf() / 1000);
  let body = `_karajo_epoch=${epoch}&id=${id}`;

  let hash = CryptoJS.HmacSHA256(body, secret);
  let sign = hash.toString(CryptoJS.enc.Hex);

  let fres = await fetch("/karajo/api/job_exec/cancel", {
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

function expandOrCollapse(event, jobID) {
  if (event) {
    event.preventDefault();
  }
  let job = _jobs[jobID];
  if (!job) {
    job = _httpJobs[jobID];
    if (!job) {
      return;
    }
  }

  let el = document.getElementById(job._idInfo);
  if (el.style.display === "block") {
    el.style.display = "none";
    job._display = "none";
  } else {
    el.style.display = "block";
    job._display = "block";
  }
  return el;
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
  headers[job.header_sign] = sign;

  let fres = await fetch(`/karajo/api/job_exec/run${jobPath}`, {
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

  let fres = await fetch("/karajo/api/job_exec/pause", {
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

  let fres = await fetch("/karajo/api/job_exec/resume", {
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
    out += `<div class="row desc">${job.description}</div><br/>`;
  }

  out += `
    <div class="row">
      <span class="key">ID:</span>
      <span class="val">${job.id}</div>
    </div>
    <div class="row">
      <span class="key">Path:</span>
      <span class="val">${job.path}</div>
    </div>
    <div class="row">
      <span class="key">Status:</span>
      <span class="val">${job.status || "-"}</span>
    </div>
    <div class="row">
      <span class="key">Last run:</span>
      <span class="val">${job._lastRun.toUTCString()}</span>
    </div>
  `;

  if (job.schedule) {
    out += `
      <div class="row">
        <span class="key">Schedule:</span>
        <span class="val">${job.schedule}</span>
      </div>
      <div class="row">
        <span class="key">Next run:</span>
        <span class="val">${job._nextRun.toUTCString()}</span>
      </div>
    `;
  } else if (job.interval > 0) {
    out += `
      <div class="row">
        <span class="key">Interval:</span>
        <span class="val">${job.interval / 1e9} seconds</span>
      </div>
      <div class="row">
        <span class="key">Next run:</span>
        <span class="val">${job._nextRun.toUTCString()}</span>
      </div>
    `;
  }

  if (job.commands) {
    out += `
      <br/>
      <div class="row job_commands">Commands:</div>
      <div class="job_commands">
    `;
    job.commands.forEach(function (cmd, idx, list) {
      out += `
        <div class="row">
          <span class="key">${idx}:</span>
          <span class="val"><tt>${cmd}</tt></span>
        </div>`;
    });
    out += `</div>`;
  }

  out += `
    <br/>
    <br/>
    <div class="row logs">Logs:</div>
    <div class="logs">
  `;

  job.logs.forEach(function (log, idx, list) {
    out += `
      <div class="row">
        <span class="key">
          <a
            href="/karajo/job_exec/log/?id=${job.id}&counter=${log.counter}"
            target="_blank"
            class="job-log ${log.status}"
          >
            #${log.counter}
          </a>
        </span>
        <span class="val">Finished at ${log._finishedAt.toUTCString()}</span>
      </div>
    `;
  });

  out += `
    </div>
    <div class="row actions">
      <button onclick="jobRunNow('${job.id}', '${job.path}')">Run now</button>
      <button onclick="jobCancel('${job.id}')">Cancel</button>
      <button onclick="jobPause('${job.id}')">Pause</button>
      <button onclick="jobResume('${job.id}')">Resume</button>
    </div>`;

  el.innerHTML = out;
}

function renderJobStatusRight(job) {
  let elNextRun = document.getElementById(job._idStatusRight);

  let now = new Date();
  let nextRun = new Date(job.next_run);

  if (nextRun <= 0) {
    if (job._lastRun > 0) {
      elNextRun.innerText = `Last run ${job._lastRun.toUTCString()}`;
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
    job._lastRun = new Date(job.last_run);
    job._nextRun = new Date(job.next_run);

    if (job.logs == null) {
      job.logs = [];
    }
    job.logs.forEach(function (log, idx, list) {
      log._finishedAt = new Date(log.finished_at);
    });

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
          <a href="#${job._id}" onclick='expandOrCollapse(event, "${job.id}")'>
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
    out += `<div class="row desc">${job.description}</div><br/>`;
  }

  out += `
    <div class="row">
      <span class="key">ID:</span>
      <span class="val">${job.id}</span>
    </div>
    <div class="row">
      <span class="key">HTTP URL:</span>
      <span class="val">${job.http_url}</span>
    </div>
  `;

  if (job.http_headers) {
    out += `
      <div class="row">
        <span class="key">HTTP headers:</span>
        <span class="val">${job.http_headers}</span>
      </div>
    `;
  }

  out += `
    <div class="row">
      <span class="key">HTTP timeout:</span>
      <span class="val">${job.http_timeout / 1e9}</span>
    </div>
  `;

  if (job.interval) {
    out += `
      <div class="row">
        <span class="key">Interval:</span>
        <span class="val">${job.interval / 1e9}s</span>
      </div>
    `;
  }

  out += `
    <div class="row">
      <span class="key">Last run:</span>
      <span class="val">${job._lastRun.toUTCString()}</span>
    </div>
    <div class="row">
      <span class="key">Next run:</span>
      <span class="val">${job._nextRun.toUTCString()}</span>
    </div>
    <div class="row">
      <span class="key">Status:</span>
      <span class="val">${job.status || ""}</span>
    </div>
  `;

  out += `
    <br/>
    <br/>
    <div class="row logs">Logs:</div>
    <div class="logs">
  `;

  job.logs.forEach(function (log, idx, list) {
    out += `
      <div class="row">
        <span class="key">
          <a
            href="/karajo/job_http/log/?id=${job.id}&counter=${log.counter}"
            target="_blank"
            class="job-log ${log.status}"
          >#${log.counter}</a>
        </span>
        <span class="val">Finished at ${log._finishedAt.toUTCString()}</span>
      </div>
    `;
  });

  out += `
    </div>
    <div class="row actions">
  `;

  if (job.status == "paused") {
    out += `<button onclick="jobHttpResume('${job.id}')">Resume</button>`;
  } else {
    out += `<button onclick="jobHttpPause('${job.id}')">Pause</button>`;
  }

  out += `</div>`;
  el.innerHTML = out;
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
    httpJob._idStatus = `jobhttp_${httpJob.id}_status`;
    httpJob._idStatusRight = `jobhttp_${httpJob.id}_status_right`;
    httpJob._display = "none";
    httpJob._lastRun = new Date(httpJob.last_run);
    httpJob._nextRun = new Date(httpJob.next_run);

    if (httpJob.logs == null) {
      httpJob.logs = [];
    }
    httpJob.logs.forEach(function (log, idx, list) {
      log._finishedAt = new Date(log.finished_at);
    });

    if (_httpJobs != null) {
      let prevJob = _httpJobs[httpJob.id];
      if (prevJob != null) {
        httpJob._display = prevJob._display;
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
          <a href="#${httpJob._id}" onclick='expandOrCollapse(event, "${httpJob.id}")'>
            ${httpJob.name}
          </a>
          <span id="${httpJob._idStatusRight}" class="status_right"></span>
        </div>

        <div id="${httpJob._idInfo}" style="display: ${httpJob._display};">
          <div id="${httpJob._idAttrs}" class="attrs"></div>
        </div>
      </div>
    `;

    elHttpJobs.innerHTML += out;
    renderJobHttp(httpJob);
  }
}

// handleFragment wait for all jobs has been rendered and expand the job
// information based on the job ID in the URL fragment.
function handleFragment(jobs, http_jobs) {
  const wait = setInterval(function () {
    // Wait for all jobs has been rendered first.
    for (let name in jobs) {
      const job = jobs[name];
      const id = `job_${job.id}`;
      let el = document.getElementById(id);
      if (!el) {
        return;
      }
    }
    for (let name in http_jobs) {
      const job = http_jobs[name];
      const id = `jobhttp_${job.id}`;
      let el = document.getElementById(id);
      if (!el) {
        return;
      }
    }

    clearInterval(wait);

    const currentUrl = new URL(window.location.href);
    let job_id = currentUrl.hash;
    if (!job_id) {
      return;
    }
    job_id = job_id.slice(1);
    if (job_id.startsWith("job_")) {
      job_id = job_id.slice(4);
    } else if (job_id.startsWith("jobhttp_")) {
      job_id = job_id.slice(8);
    } else {
      return;
    }
    const el = expandOrCollapse(null, job_id);
    el.scrollIntoView();
  }, 500);
}
