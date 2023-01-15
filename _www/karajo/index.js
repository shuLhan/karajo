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

  renderJobs(_env.Jobs);
  renderHttpJobs(_env.HttpJobs);
}

function setTitle() {
  document.title = _env.Name;
  document.getElementById("title").innerHTML = _env.Name;
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
  let secret = document.getElementById("_secret").value;
  let epoch = parseInt(new Date().valueOf() / 1000);
  let req = {
    _karajo_epoch: epoch,
  };
  let body = JSON.stringify(req);

  let hash = CryptoJS.HmacSHA256(body, secret);
  let sign = hash.toString(CryptoJS.enc.Hex);

  let fres = await fetch(`/karajo/api/job/run${jobPath}`, {
    method: "POST",
    headers: {
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
  } else {
    el.style.display = "block";
    job._display = "block";
  }
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
  renderJobLastRun(job);
  renderJobStatus(job);
}

function renderJobAttributes(job) {
  let el = document.getElementById(job._idAttrs);
  let out = `
    <div>${job.Description}</div>
    <br/>
    <div>ID: ${job.ID}</div>
    <div>Path: ${job.Path}</div>
    <div>Status: ${job.Status}</div>
    <div>Last run: ${job.LastRun}</div>
  `;

  if (job.Interval > 0) {
    out += `
      <div>Interval: ${job.Interval / 1e9} seconds</div>
      <div>Next run: ${job.NextRun}</div>
    `;
  }

  out += `
    <br/>
    <div class="job_commands">
      Commands:
  `;

  job.Commands.forEach(function (cmd, idx, list) {
    out += `<div> ${idx}: <tt>${cmd}</tt> </div>`;
  });

  out += `
    </div>
    <br/>
    <div>Log:</div>
    <div>
  `;

  if (job.Logs == null) {
    job.Logs = [];
  }

  job.Logs.forEach(function (log, idx, list) {
    out += `<a
      href="/karajo/job/log?id=${job.ID}&counter=${log.Counter}"
      target="_blank"
      class="job-log ${log.Status}"
    >
        #${log.Counter}
    </a>`;
  });

  out += `&nbsp;<button onclick="jobRunNow('${job.ID}', '${job.Path}')">Run now</button>`;
  out += `&nbsp;<button onclick="jobPause('${job.ID}')">Pause</button>`;
  out += `&nbsp;<button onclick="jobResume('${job.ID}')">Resume</button>`;

  out += "</div>";

  el.innerHTML = out;
}

function renderJobLastRun(job) {
  let elLastRun = document.getElementById(job._idLastRun);

  let now = new Date();
  let lastRun = new Date(job.LastRun);

  if (lastRun <= 0) {
    if (job.Status != "") {
      elLastRun.innerText = "Running ...";
    }
    return;
  }

  let seconds = Math.floor((now - lastRun) / 1000);
  let hours = Math.floor(seconds / 3600);
  let minutes = Math.floor((seconds % 3600) / 60);
  let remSeconds = Math.floor(seconds % 60);
  elLastRun.innerText = `Last run ${hours}h  ${minutes}m ${remSeconds}s ago`;
}

function renderJobStatus(job) {
  let el = document.getElementById(job._idStatus);
  el.className = `name ${job.Status}`;
}

// renderJobs render list of jobs.
function renderJobs(jobs) {
  let elJobs = document.getElementById("jobs");

  for (let name in jobs) {
    let job = jobs[name];

    job._id = `job_${job.ID}`;
    job._idAttrs = `job_${job.ID}_attrs`;
    job._idInfo = `job_${job.ID}_info`;
    job._idLastRun = `job_${job.ID}_last_run`;
    job._idStatus = `job_${job.ID}_status`;
    job._display = "none";

    if (_jobs != null) {
      let prevJob = _jobs[job.ID];
      if (prevJob != null) {
        job._display = prevJob._display;
      }
    }

    _jobs[job.ID] = job;

    let elJobInfo = document.getElementById(job._idInfo);
    if (elJobInfo != null) {
      renderJob(job);
      continue;
    }

    let out = `
      <div id="${job._id}" class="job">
        <div id="${job._idStatus}" class="name ${job.Status}">
          <a href="#${job._id}" onclick='jobInfo("${job.ID}")'>
            ${job.Name}
          </a>
          <span id="${job._idLastRun}" class="last_run"></span>
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
  renderJobHttpLog(job);
  renderJobHttpNextRun(job);
  renderJobHttpStatus(job);
}

function renderJobHttpAttrs(job) {
  let el = document.getElementById(job._idAttrs);
  let out = `
    <div>${job.Description}</div>
    <br/>
    <div>ID: ${job.ID}</div>
    <div>HTTP URL: ${job.HttpUrl}</div>
    <div>HTTP headers: ${job.HttpHeaders}</div>
    <div>HTTP timeout: ${job.HttpTimeout / 1e9}</div>
    <div>Interval: ${job.Interval / 1e9}s</div>
    <div>Maximum job running: ${job.MaxRunning}</div>
    <div>Currently job running: ${job.NumRunning}</div>
    <div>Last run: ${job.LastRun}</div>
    <div>Next run: ${job.NextRun}</div>
    <div>Status: ${job.Status}</div>
    <br/>
    <div class="actions">
  `;

  if (job.Status == "paused") {
    out += `<button onclick="jobHttpResume('${job.ID}')">Resume</button>`;
  } else {
    out += `<button onclick="jobHttpPause('${job.ID}')">Pause</button>`;
  }

  out += `</div>`;
  el.innerHTML = out;
}

function renderJobHttpLog(job) {
  let el = document.getElementById(job._idLog);
  let out = "";

  for (let x = 0; x < job.Log.length; x++) {
    out += `<p>${job.Log[x]}</p>`;
  }

  el.innerHTML = out;
  el.scrollTop = el.scrollHeight;
}

function renderJobHttpNextRun(job) {
  let elNextRun = document.getElementById(job._idNextRun);

  let now = new Date();
  let nextRun = new Date(job.NextRun);

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
  el.className = `name ${job.Status}`;
}

function renderHttpJobs(httpJobs) {
  let out = "";
  let elHttpJobs = document.getElementById("http_jobs");

  for (let name in httpJobs) {
    let httpJob = httpJobs[name];

    httpJob._id = `jobhttp_${httpJob.ID}`;
    httpJob._idAttrs = `jobhttp_${httpJob.ID}_attrs`;
    httpJob._idInfo = `jobhttp_${httpJob.ID}_info`;
    httpJob._idLog = `jobhttp_${httpJob.ID}_log`;
    httpJob._idStatus = `jobhttp_${httpJob.ID}_status`;
    httpJob._idNextRun = `jobhttp_${httpJob.ID}_next_run`;
    httpJob._display = "none";

    if (_httpJobs != null) {
      let prevJob = _httpJobs[httpJob.ID];
      if (prevJob != null) {
        httpJob._display = prevJob._display;
      }
    }

    _httpJobs[httpJob.ID] = httpJob;

    let elJob = document.getElementById(httpJob._id);
    if (elJob != null) {
      renderJobHttp(httpJob);
      continue;
    }

    out = `
      <div id="${httpJob._id}" class="jobhttp">
        <div id="${httpJob._idStatus}" class="name ${httpJob.Status}">
          <a href="#${httpJob._id}" onclick='jobHttpInfo("${httpJob.ID}")'>
            ${httpJob.Name}
          </a>
          <span id="${httpJob._idNextRun}" class="next_run"></span>
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
