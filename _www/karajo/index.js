// SPDX-FileCopyrightText: 2022 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

let _env = {};
let _jobs = {};
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
  renderJobs(_env.Jobs);
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

async function jobPause(id) {
  let secret = document.getElementById("_secret").value;
  let epoch = parseInt(new Date().valueOf() / 1000);
  let q = `_karajo_epoch=${epoch}&id=${id}`;

  let hash = CryptoJS.HmacSHA256(q, secret);
  let sign = hash.toString(CryptoJS.enc.Hex);

  let fres = await fetch("/karajo/api/job/pause?" + q, {
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

  let job = _jobs[id];
  job = Object.assign(job, res.data);
  renderJob(job);
}

async function jobResume(id) {
  let secret = document.getElementById("_secret").value;
  let epoch = parseInt(new Date().valueOf() / 1000);
  let q = `_karajo_epoch=${epoch}&id=${id}`;

  let hash = CryptoJS.HmacSHA256(q, secret);
  let sign = hash.toString(CryptoJS.enc.Hex);

  let fres = await fetch("/karajo/api/job/resume?" + q, {
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

  let job = _jobs[id];
  job = Object.assign(job, res.data);
  renderJob(job);
}

// renderHook render single hook.
function renderHook(hook) {
  renderHookAttributes(hook);
  renderHookStatus(hook);
}

function renderHookAttributes(hook) {
  let el = document.getElementById(hook._idAttrs);
  let out = `
    <div>${hook.Description}</div>
    <br/>
    <div>ID: ${hook.ID}</div>
    <div>Path: ${hook.Path}</div>
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
    return;
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

  out += "</div>";

  el.innerHTML = out;
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

function renderJob(job) {
  renderJobAttrs(job);
  renderJobLog(job);
  renderJobNextRun(job);
  renderJobStatus(job);
}

function renderJobAttrs(job) {
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
    out += `<button onclick="jobResume('${job.ID}')">Resume</button>`;
  } else {
    out += `<button onclick="jobPause('${job.ID}')">Pause</button>`;
  }

  out += `</div>`;
  el.innerHTML = out;
}

function renderJobLog(job) {
  let el = document.getElementById(job._idLog);
  let out = "";

  for (let x = 0; x < job.Log.length; x++) {
    out += `<p>${job.Log[x]}</p>`;
  }

  el.innerHTML = out;
  el.scrollTop = el.scrollHeight;
}

function renderJobNextRun(job) {
  let now = new Date();
  let nextRun = new Date(job.NextRun);
  let seconds = Math.floor((nextRun - now) / 1000);
  let hours = Math.floor(seconds / 3600);
  let minutes = Math.floor((seconds % 3600) / 60);
  let remSeconds = Math.floor(seconds % 60);
  let elNextRun = document.getElementById(job._idNextRun);
  elNextRun.innerText = `Next run in ${hours}h ${minutes}m ${remSeconds}s`;
}

function renderJobStatus(job) {
  let el = document.getElementById(job._idStatus);
  el.className = `name ${job.Status}`;
}

function renderJobs(jobs) {
  let out = "";
  let elJobs = document.getElementById("jobs");

  for (let name in jobs) {
    let job = jobs[name];

    job._id = `job_${job.ID}`;
    job._idAttrs = `job_${job.ID}_attrs`;
    job._idInfo = `job_${job.ID}_info`;
    job._idLog = `job_${job.ID}_log`;
    job._idStatus = `job_${job.ID}_status`;
    job._idNextRun = `job_${job.ID}_next_run`;
    job._display = "none";

    if (_jobs != null) {
      let prevJob = _jobs[job.ID];
      if (prevJob != null) {
        job._display = prevJob._display;
      }
    }

    _jobs[job.ID] = job;

    let elJob = document.getElementById(job._id);
    if (elJob != null) {
      renderJob(job);
      continue;
    }

    out = `
      <div id="${job._id}" class="job">
        <div id="${job._idStatus}" class="name ${job.Status}">
          <a href="#${job._id}" onclick='jobInfo("${job.ID}")'>
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
    renderJob(job);
  }
}
