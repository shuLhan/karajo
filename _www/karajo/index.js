// SPDX-FileCopyrightText: 2022 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

let _env = {};
let _httpJobs = {};
let _hooks = {};

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

  renderHooks(_env.Hooks);
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

async function hookInfo(hookID) {
  let hook = _hooks[hookID];
  let el = document.getElementById(hook._idInfo);
  let delay = 10000;

  if (el.style.display === "block") {
    el.style.display = "none";
    hook._display = "none";
  } else {
    el.style.display = "block";
    hook._display = "block";
  }
}

async function hookRunNow(hookID, hookPath) {
  let secret = document.getElementById("_secret").value;
  let epoch = parseInt(new Date().valueOf() / 1000);
  let req = {
    _karajo_epoch: epoch,
  };
  let body = JSON.stringify(req);

  let hash = CryptoJS.HmacSHA256(body, secret);
  let sign = hash.toString(CryptoJS.enc.Hex);

  let fres = await fetch(`/karajo/hook${hookPath}`, {
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

  let hook = res.data;
  renderHooks([hook]);
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

// renderHook render single hook.
function renderHook(hook) {
  renderHookAttributes(hook);
  renderHookLastRun(hook);
  renderHookStatus(hook);
}

function renderHookAttributes(hook) {
  let el = document.getElementById(hook._idAttrs);
  let out = `
    <div>${hook.Description}</div>
    <br/>
    <div>ID: ${hook.ID}</div>
    <div>Path: ${hook.Path}</div>
    <div>Last run: ${hook.LastRun}</div>
    <div class="hook_commands">
      Commands:
  `;

  hook.Commands.forEach(function (cmd, idx, list) {
    out += `<div> ${idx}: <tt>${cmd}</tt> </div>`;
  });

  out += `
    </div>
    <div>Log:</div>
    <div>
  `;

  if (hook.Logs == null) {
    hook.Logs = [];
  }

  hook.Logs.forEach(function (log, idx, list) {
    out += `<a
      href="/karajo/hook/log?id=${hook.ID}&counter=${log.Counter}"
      target="_blank"
      class="hook-log ${log.Status}"
    >
        #${log.Counter}
    </a>`;
  });

  out += `&nbsp;<button onclick="hookRunNow('${hook.ID}', '${hook.Path}')">Run now</button>`;

  out += "</div>";

  el.innerHTML = out;
}

function renderHookLastRun(hook) {
  let elLastRun = document.getElementById(hook._idLastRun);

  let now = new Date();
  let lastRun = new Date(hook.LastRun);

  if (lastRun <= 0) {
    if (hook.LastStatus != "") {
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

function renderHookStatus(hook) {
  let el = document.getElementById(hook._idStatus);
  el.className = `name ${hook.LastStatus}`;
}

// renderHooks render list of hooks.
function renderHooks(hooks) {
  let elHooks = document.getElementById("hooks");

  for (let name in hooks) {
    let hook = hooks[name];

    hook._id = `hook_${hook.ID}`;
    hook._idAttrs = `hook_${hook.ID}_attrs`;
    hook._idInfo = `hook_${hook.ID}_info`;
    hook._idLastRun = `hook_${hook.ID}_last_run`;
    hook._idStatus = `hook_${hook.ID}_status`;
    hook._display = "none";

    if (_hooks != null) {
      let prevHook = _hooks[hook.ID];
      if (prevHook != null) {
        hook._display = prevHook._display;
      }
    }

    _hooks[hook.ID] = hook;

    let elHookInfo = document.getElementById(hook._idInfo);
    if (elHookInfo != null) {
      renderHook(hook);
      continue;
    }

    let out = `
      <div id="${hook._id}" class="hook">
        <div id="${hook._idStatus}" class="name ${hook.LastStatus}">
          <a href="#${hook._id}" onclick='hookInfo("${hook.ID}")'>
            ${hook.Name}
          </a>
          <span id="${hook._idLastRun}" class="last_run"></span>
        </div>

        <div id="${hook._idInfo}" style="display: ${hook._display};">
          <div id="${hook._idAttrs}" class="attrs"></div>
        </div>
      </div>
    `;

    let elHook = document.createElement("div");
    elHook.innerHTML = out;
    elHooks.appendChild(elHook);

    renderHook(hook);
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
    <div>Number of requests: ${job.NumRequests}</div>
    <div>Maximum requests: ${job.MaxRequests}</div>
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

function renderHttpJobs(jobs) {
  let out = "";
  let elJobs = document.getElementById("http_jobs");

  for (let name in jobs) {
    let job = jobs[name];

    job._id = `job_${job.ID}`;
    job._idAttrs = `job_${job.ID}_attrs`;
    job._idInfo = `job_${job.ID}_info`;
    job._idLog = `job_${job.ID}_log`;
    job._idStatus = `job_${job.ID}_status`;
    job._idNextRun = `job_${job.ID}_next_run`;
    job._display = "none";

    if (_httpJobs != null) {
      let prevJob = _httpJobs[job.ID];
      if (prevJob != null) {
        job._display = prevJob._display;
      }
    }

    _httpJobs[job.ID] = job;

    let elJob = document.getElementById(job._id);
    if (elJob != null) {
      renderJobHttp(job);
      continue;
    }

    out = `
      <div id="${job._id}" class="job">
        <div id="${job._idStatus}" class="name ${job.Status}">
          <a href="#${job._id}" onclick='jobHttpInfo("${job.ID}")'>
            ${job.Name}
          </a>
          <span id="${job._idNextRun}" class="next_run"></span>
        </div>

        <div id="${job._idInfo}" style="display: ${job._display};">
          <div id="${job._idAttrs}" class="attrs"></div>
          <div id="${job._idLog}" class="log"></div>
        </div>
      </div>
    `;

    elJobs.innerHTML += out;
    renderJobHttp(job);
  }
}
