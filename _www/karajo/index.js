// SPDX-FileCopyrightText: 2022 M. Shulhan <ms@kilabit.info>
// SPDX-License-Identifier: GPL-3.0-or-later

let _env = {};
let _jobs = {};
let _hooks = {};

async function main() {
  runTimer();

  let wout = document.getElementById("out");
  let werr = document.getElementById("err");

  wout.innerHTML = "";
  werr.innerHTML = "";

  let fres = await fetch("/karajo/api/environment");
  let res = await fres.json();
  if (res.code != 200) {
    werr.innerHTML = res.message;
    return;
  }

  _env = res.data;
  setTitle();

  renderHooks(_env.Hooks);

  let jobs = res.data.Jobs;
  let out = "";
  for (let x = 0; x < jobs.length; x++) {
    let job = jobs[x];
    _jobs[job.ID] = job;

    job._idAttrs = job.ID + "_attrs";
    job._idInfo = job.ID + "_info";
    job._idLog = job.ID + "_log";
    job._idStatus = job.ID + "_status";

    out += `
      <div class="job">
        <div id="${job._idStatus}" class="name ${job.Status}">
          <a href="#${job.ID}" onclick='jobInfo("${job.ID}")'>
            ${job.Name}
          </a>
        </div>

        <div id="${job._idInfo}" style="display: none;">
          <div id="${job._idAttrs}" class="attrs"></div>
          <div id="${job._idLog}" class="log"></div>
        </div>
      </div>
    `;
  }

  document.getElementById("jobs").innerHTML = out;

  for (var jobID in _jobs) {
    console.log("render job ", _jobs[jobID]);
    renderJob(_jobs[jobID]);
  }
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
  } else {
    el.style.display = "block";
  }
}

async function jobInfo(jobID) {
  let job = _jobs[jobID];
  let el = document.getElementById(job._idInfo);
  let delay = 10000;

  if (el.style.display === "block") {
    el.style.display = "none";
    window.clearInterval(job._refreshInterval);
    console.log(`${job.ID}: pooling job stopped.`);
  } else {
    el.style.display = "block";

    await jobRefresh(job);

    console.log(
      `${job.ID}: started pooling job every ${delay / 1000} seconds.`
    );

    job._refreshInterval = setInterval(() => {
      jobRefresh(job);
    }, delay);
  }
}

async function jobRefresh(job) {
  let fres = await fetch("/karajo/api/job?id=" + job.ID);
  let res = await fres.json();
  if (res.code !== 200) {
    console.error(res.message);
    return;
  }

  job = Object.assign(job, res.data);
  renderJob(job);
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

// renderHooks render single hook.
function renderHook(hook) {
  renderHookAttributes(hook);
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

// renderHooks render list of hooks.
function renderHooks(hooks) {
  let el = document.getElementById("hooks");
  let out = "";

  for (let name in hooks) {
    let hook = hooks[name];

    hook._idInfo = `hook_${hook.ID}_info`;
    hook._idAttrs = `hook_${hook.ID}_attrs`;
    hook._idLog = `hook_${hook.ID}_log`;

    _hooks[hook.ID] = hook;

    out += `
      <div class="hook">
        <div id="${hook.ID}" class="name ${hook.Status}">
          <a href="#${hook.ID}" onclick='hookInfo("${hook.ID}")'>
            ${hook.Name}
          </a>
        </div>

        <div id="${hook._idInfo}" style="display: none;">
          <div id="${hook._idAttrs}" class="attrs"></div>
        </div>
      </div>
    `;
  }

  el.innerHTML = out;

  for (var id in _hooks) {
    console.log("render hook:", _hooks[id]);
    renderHook(_hooks[id]);
  }
}

function renderJob(job) {
  renderJobAttrs(job);
  renderJobLog(job);
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

function renderJobStatus(job) {
  let el = document.getElementById(job._idStatus);
  el.className = `name ${job.Status}`;
}
