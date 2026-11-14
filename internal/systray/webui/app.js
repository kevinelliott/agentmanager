"use strict";

// AgentManager desktop UI. No build step and no dependencies: the helper
// embeds these files as they are and serves them on loopback.

const ICONS = {
  installed: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><rect x="2" y="2" width="5" height="5" rx="1.2"/><rect x="9" y="2" width="5" height="5" rx="1.2"/><rect x="2" y="9" width="5" height="5" rx="1.2"/><rect x="9" y="9" width="5" height="5" rx="1.2"/></svg>',
  updates: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M8 13V3M4 7l4-4 4 4"/></svg>',
  catalog: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M2.5 3.5h11M2.5 8h11M2.5 12.5h11"/></svg>',
  activity: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M1.5 8h3l2-5 3 10 2-5h3"/></svg>',
  settings: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M2 4.5h7M12 4.5h2M2 11.5h2M7 11.5h7"/><circle cx="10.5" cy="4.5" r="1.6"/><circle cx="5.5" cy="11.5" r="1.6"/></svg>',
  search: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"><circle cx="7" cy="7" r="4.5"/><path d="M10.5 10.5 14 14"/></svg>',
  refresh: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M13.5 8a5.5 5.5 0 1 1-1.6-3.9M13.5 2.5v2.6h-2.6"/></svg>',
  back: '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><path d="M9.5 3.5 5 8l4.5 4.5"/></svg>',
  done: '<svg viewBox="0 0 18 18" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><circle cx="9" cy="9" r="7"/><path d="m6 9.2 2.1 2.1L12 7.2"/></svg>',
  failed: '<svg viewBox="0 0 18 18" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round"><circle cx="9" cy="9" r="7"/><path d="M9 5.5v4M9 12.4v.1"/></svg>',
  queued: '<svg viewBox="0 0 18 18" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round"><circle cx="9" cy="9" r="7"/><path d="M9 5.5V9l2.3 1.5"/></svg>',
};

const state = {
  ready: false,
  data: null, // /api/state
  catalog: null, // /api/catalog agents
  catalogError: "",
  byId: new Map(),
  query: "",
  category: "all",
  route: { view: "installed", id: "" },
  methodChoice: {}, // agent id -> chosen install method
  wasBusy: false,
};

const $ = (sel) => document.querySelector(sel);

function esc(s) {
  return String(s ?? "").replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
}

function hue(s) {
  let h = 0;
  for (let i = 0; i < s.length; i++) h = (h * 31 + s.charCodeAt(i)) >>> 0;
  return h % 360;
}

function avatar(name, id, large) {
  const letter = (name || "?").trim().charAt(0) || "?";
  return `<span class="avatar${large ? " lg" : ""}" style="--h:${hue(id || name)}" aria-hidden="true">${esc(letter)}</span>`;
}

function plural(n, one, many) {
  return `${n} ${n === 1 ? one : many || one + "s"}`;
}

function ago(iso) {
  if (!iso) return "";
  const t = new Date(iso).getTime();
  if (!t || t < 1e12) return "";
  const s = Math.max(0, (Date.now() - t) / 1000);
  if (s < 10) return "just now";
  if (s < 60) return `${Math.floor(s)}s ago`;
  if (s < 3600) return `${Math.floor(s / 60)} min ago`;
  if (s < 86400) return `${Math.floor(s / 3600)} hr ago`;
  return new Date(t).toLocaleDateString();
}

function toast(msg, isError) {
  const el = document.createElement("div");
  el.className = "toast" + (isError ? " error" : "");
  el.textContent = msg;
  $("#toasts").appendChild(el);
  setTimeout(() => el.remove(), isError ? 7000 : 3500);
}

// The embedded web views do not show window.confirm(), so destructive
// actions are confirmed with an in-page dialog.
function confirmDialog(title, message, okLabel) {
  return new Promise((resolve) => {
    const dlg = document.createElement("dialog");
    dlg.className = "dialog";
    dlg.innerHTML = `<h2>${esc(title)}</h2><p>${esc(message)}</p><div class="dialog-actions"><button class="btn" value="cancel" autofocus>Cancel</button><button class="btn primary destructive" value="ok">${esc(okLabel)}</button></div>`;
    dlg.addEventListener("click", (e) => {
      const b = e.target.closest("button");
      if (b) dlg.close(b.value);
      else if (e.target === dlg) dlg.close("cancel");
    });
    dlg.addEventListener("close", () => {
      dlg.remove();
      resolve(dlg.returnValue === "ok");
    });
    document.body.appendChild(dlg);
    dlg.showModal();
  });
}

// ---------- API ----------

async function api(path, body) {
  const opts = body === undefined ? {} : {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  };
  const res = await fetch(path, opts);
  let payload = null;
  try { payload = await res.json(); } catch { /* empty body */ }
  if (!res.ok) {
    throw new Error((payload && payload.error) || `Request failed (${res.status})`);
  }
  return payload;
}

async function post(path, body, okMessage) {
  try {
    const out = await api(path, body || {});
    if (okMessage) toast(okMessage);
    await loadState();
    return out;
  } catch (err) {
    toast(err.message, true);
    return null;
  }
}

async function loadState() {
  try {
    state.data = await api("/api/state");
    state.offline = false;
  } catch (err) {
    state.offline = true;
  }
  state.ready = true;

  // When work finishes, the set of installable things may have changed.
  const busy = isBusy();
  if (state.wasBusy && !busy) loadCatalog();
  state.wasBusy = busy;

  render();
}

async function loadCatalog() {
  try {
    const out = await api("/api/catalog");
    state.catalog = out.agents || [];
    state.byId = new Map(state.catalog.map((a) => [a.id, a]));
    state.catalogError = "";
  } catch (err) {
    state.catalogError = err.message;
  }
  render();
}

function isBusy() {
  const d = state.data;
  return !!d && (d.refreshing || d.jobs.some((j) => j.status === "queued" || j.status === "running"));
}

function poll() {
  loadState().finally(() => setTimeout(poll, isBusy() ? 900 : 3000));
}

// ---------- derived data ----------

function installs() { return (state.data && state.data.installs) || []; }
function jobs() { return (state.data && state.data.jobs) || []; }
function updates() { return installs().filter((i) => i.has_update); }

function pendingJob(agentId, method) {
  return jobs().find((j) => j.agent_id === agentId && (!method || j.method === method) &&
    (j.status === "queued" || j.status === "running"));
}

function installsOf(agentId) { return installs().filter((i) => i.agent_id === agentId); }

function matches(text) {
  const q = state.query.trim().toLowerCase();
  if (!q) return true;
  return q.split(/\s+/).every((w) => text.includes(w));
}

function catalogText(a) {
  return `${a.name} ${a.id} ${a.description} ${a.category || ""} ${(a.tags || []).join(" ")} ${a.vendor || ""}`.toLowerCase();
}

// ---------- routing ----------

function parseRoute() {
  const parts = location.hash.replace(/^#\/?/, "").split("/");
  const view = ["installed", "updates", "catalog", "activity", "settings", "agent"].includes(parts[0]) ? parts[0] : "installed";
  return { view, id: decodeURIComponent(parts.slice(1).join("/")) };
}

function go(path) { location.hash = path; }

window.addEventListener("hashchange", () => {
  const prev = state.route.view;
  state.route = parseRoute();
  if (prev !== state.route.view && state.route.view !== "agent") {
    state.query = "";
  }
  shellKey = "";
  render();
  $("#main").focus({ preventScroll: true });
});

// ---------- rendering ----------

// The view's frame (title, search box) is rebuilt only when the route
// changes; data refreshes replace just the body, so typing in the search box
// and scroll position survive the polling.
let shellKey = "";
let lastBody = "";
let lastNav = "";

function setHTML(el, html, cacheName) {
  if (cacheName === "body" && html === lastBody) return;
  if (cacheName === "nav" && html === lastNav) return;
  if (cacheName === "body") lastBody = html;
  if (cacheName === "nav") lastNav = html;
  el.innerHTML = html;
}

function render() {
  renderNav();
  const r = state.route;
  const key = r.view + ":" + r.id;
  if (key !== shellKey) {
    shellKey = key;
    lastBody = "";
    $("#main").innerHTML = shell(r);
  }
  const body = $("#body");
  if (body) setHTML(body, bodyHTML(r), "body");
  const sub = $("#head-sub");
  if (sub) sub.textContent = headSub(r);
  const actions = $("#head-actions");
  if (actions) {
    const html = headActions(r);
    if (actions.dataset.html !== html) {
      actions.dataset.html = html;
      actions.innerHTML = html;
    }
  }
  const filters = $("#filters");
  if (filters) {
    const html = filtersHTML();
    if (filters.dataset.html !== html) {
      filters.dataset.html = html;
      filters.innerHTML = html;
    }
  }
}

function renderNav() {
  const d = state.data;
  const active = state.route.view === "agent" ? "" : state.route.view;
  const nUpdates = updates().length;
  const nBusy = jobs().filter((j) => j.status === "queued" || j.status === "running").length;
  const items = [
    ["installed", "Installed", d ? `<span class="nav-count">${installs().length}</span>` : ""],
    ["updates", "Updates", nUpdates ? `<span class="nav-badge">${nUpdates}</span>` : ""],
    ["catalog", "Catalog", state.catalog ? `<span class="nav-count">${state.catalog.length}</span>` : ""],
    ["activity", "Activity", nBusy ? `<span class="nav-badge busy">${nBusy}</span>` : ""],
  ];
  let html = items.map(([id, label, extra]) =>
    `<button class="nav-item" data-go="/${id}"${active === id ? ' aria-current="page"' : ""}>${ICONS[id]}<span class="nav-label">${label}</span>${extra}</button>`).join("");
  html += `<div class="nav-sep"></div>`;
  html += `<button class="nav-item" data-go="/settings"${active === "settings" ? ' aria-current="page"' : ""}>${ICONS.settings}<span class="nav-label">Settings</span></button>`;
  setHTML($("#nav"), html, "nav");

  const foot = $("#sidebar-foot");
  const footText = !d ? "" : d.refreshing ? "Checking…" : (ago(d.last_refresh) ? `Checked ${ago(d.last_refresh)}` : "");
  foot.innerHTML = d ? `<span>${esc(footText)}</span><span>Version ${esc(d.version)}</span>` : "";
}

const TITLES = { installed: "Installed", updates: "Updates", catalog: "Catalog", activity: "Activity", settings: "Settings" };

function shell(r) {
  if (r.view === "agent") {
    return `<div class="head"><button class="btn quiet back" data-act="back">${ICONS.back}Back</button><span class="head-spacer"></span><span id="head-actions"></span></div><div class="scroll" id="body"></div>`;
  }
  const searchable = r.view === "installed" || r.view === "catalog";
  const search = searchable ? `<label class="search">${ICONS.search}<input id="search" type="search" placeholder="${r.view === "catalog" ? "Search the catalog" : "Search installed agents"}" value="${esc(state.query)}" autocomplete="off" spellcheck="false" aria-label="Search"></label>` : "";
  const filters = r.view === "catalog" ? `<div class="filters" id="filters"></div>` : "";
  return `<div class="head"><h1>${TITLES[r.view]}</h1><span class="head-sub" id="head-sub"></span><span class="head-spacer"></span>${search}<span id="head-actions" class="row-side"></span></div>${filters}<div class="scroll" id="body"></div>`;
}

function headSub(r) {
  if (!state.data) return "";
  if (r.view === "installed") return plural(installs().length, "agent");
  if (r.view === "updates") { const n = updates().length; return n ? `${n} available` : ""; }
  if (r.view === "catalog" && state.catalog) return `${plural(state.catalog.length, "agent")} available`;
  return "";
}

function headActions(r) {
  const d = state.data;
  if (!d) return "";
  const refresh = `<button class="btn" data-act="refresh"${d.refreshing ? " disabled" : ""}>${d.refreshing ? '<span class="spinner"></span>Checking…' : ICONS.refresh + "Check Now"}</button>`;
  if (r.view === "installed") return refresh;
  if (r.view === "updates") {
    const n = updates().filter((i) => !pendingJob(i.agent_id, i.method)).length;
    return refresh + (updates().length ? `<button class="btn primary" data-act="update-all"${n ? "" : " disabled"}>Update All</button>` : "");
  }
  if (r.view === "activity") {
    const finished = jobs().some((j) => j.status === "done" || j.status === "failed");
    return finished ? `<button class="btn" data-act="clear-jobs">Clear Finished</button>` : "";
  }
  return "";
}

function filtersHTML() {
  if (!state.catalog) return "";
  const counts = { all: state.catalog.length };
  for (const a of state.catalog) {
    const c = a.category || "other";
    counts[c] = (counts[c] || 0) + 1;
  }
  const order = ["all", "coding", "terminal", "devops", "research", "productivity", "other"].filter((c) => counts[c]);
  return order.map((c) => `<button class="chip" data-cat="${c}" aria-pressed="${state.category === c}">${c}<span class="n">${counts[c]}</span></button>`).join("");
}

function bodyHTML(r) {
  if (!state.ready) return `<div class="empty"><span class="spinner"></span></div>`;
  if (!state.data) {
    return `<div class="empty"><h2>AgentManager is not responding</h2><p>The background helper may have quit. Reopen AgentManager from your applications to continue.</p></div>`;
  }
  switch (r.view) {
    case "installed": return installedHTML();
    case "updates": return updatesHTML();
    case "catalog": return catalogHTML();
    case "activity": return activityHTML();
    case "settings": return settingsHTML();
    case "agent": return agentHTML(r.id);
  }
  return "";
}

function versionHTML(i) {
  const v = esc(i.version && i.version !== "0" ? i.version : "unknown");
  if (i.has_update) return `<span class="ver" title="${v} → ${esc(i.latest)}">${v}<span class="arrow">→</span><span class="new">${esc(i.latest)}</span></span>`;
  return `<span class="ver">${v}</span>`;
}

function installAction(i) {
  const job = pendingJob(i.agent_id, i.method);
  if (job) return working(job);
  if (i.has_update) return `<button class="btn primary" data-act="update" data-id="${esc(i.agent_id)}" data-method="${esc(i.method)}">Update</button>`;
  return upToDate(i);
}

// Only claim "up to date" when a latest version is actually known. Some
// install methods have no registry to compare against.
function upToDate(i) {
  return i.latest ? `<span class="tag ok">Up to date</span>` : "";
}

function working(job) {
  const verb = job.status === "queued" ? "Waiting" : { install: "Installing", update: "Updating", uninstall: "Removing" }[job.action];
  return `<span class="working"><span class="spinner"></span>${verb}…</span>`;
}

function installRow(i) {
  const def = state.byId.get(i.agent_id);
  const desc = def ? def.description : i.path;
  return `<div class="row link" data-go="/agent/${encodeURIComponent(i.agent_id)}">
    ${avatar(i.name, i.agent_id)}
    <div class="row-main">
      <div class="row-title"><span class="name">${esc(i.name)}</span><span class="tag">${esc(i.method)}</span></div>
      <div class="row-desc">${esc(desc || "")}</div>
    </div>
    <div class="row-side">${versionHTML(i)}${installAction(i)}</div>
  </div>`;
}

function installedHTML() {
  const all = installs();
  if (!all.length) {
    if (state.data.refreshing) return `<div class="empty"><span class="spinner"></span><p>Looking for agents on this computer…</p></div>`;
    return `<div class="empty"><h2>No agents found yet</h2><p>AgentManager looks for AI coding agents installed on this computer. Install one from the catalog to get started.</p><button class="btn primary" data-go="/catalog">Browse Catalog</button></div>`;
  }
  const shown = all.filter((i) => matches(`${i.name} ${i.agent_id} ${i.method}`.toLowerCase()));
  if (!shown.length) return noMatches();
  const n = updates().length;
  const banner = n && !state.query ? `<div class="banner"><span class="grow">${plural(n, "update")} available</span><button class="btn" data-go="/updates">Review</button></div>` : "";
  return banner + `<div class="list">${shown.map(installRow).join("")}</div>`;
}

function updatesHTML() {
  const list = updates();
  if (!list.length) {
    return `<div class="empty"><h2>Everything is up to date</h2><p>${state.data.last_refresh && ago(state.data.last_refresh) ? "Last checked " + ago(state.data.last_refresh) + "." : ""}</p></div>`;
  }
  return `<div class="list">${list.map(installRow).join("")}</div>`;
}

function noMatches() {
  return `<div class="empty"><h2>No matches</h2><p>Nothing matches “${esc(state.query)}”.</p><button class="btn" data-act="clear-search">Clear Search</button></div>`;
}

function chosenMethod(a) {
  const usable = a.methods.filter((m) => m.available);
  const pick = state.methodChoice[a.id];
  if (pick && a.methods.some((m) => m.name === pick)) return pick;
  return usable.length ? usable[0].name : (a.methods[0] ? a.methods[0].name : "");
}

function catalogAction(a) {
  const mine = installsOf(a.id);
  const job = pendingJob(a.id);
  if (job) return working(job);
  if (mine.length) return `<span class="tag ok">Installed</span>`;
  const usable = a.methods.filter((m) => m.available);
  if (!usable.length) return `<span class="tag plain">${a.methods.length ? "Needs " + esc(a.methods.map((m) => m.name).join(" or ")) : "Not available here"}</span>`;
  const select = usable.length > 1
    ? `<select class="select" data-method-for="${esc(a.id)}" aria-label="Install method">${usable.map((m) => `<option${m.name === chosenMethod(a) ? " selected" : ""}>${esc(m.name)}</option>`).join("")}</select>`
    : `<span class="tag">${esc(usable[0].name)}</span>`;
  return `${select}<button class="btn" data-act="install" data-id="${esc(a.id)}">Install</button>`;
}

function catalogRow(a) {
  return `<div class="row link" data-go="/agent/${encodeURIComponent(a.id)}">
    ${avatar(a.name, a.id)}
    <div class="row-main">
      <div class="row-title"><span class="name">${esc(a.name)}</span></div>
      <div class="row-desc">${esc(a.description)}</div>
    </div>
    <div class="row-side">${catalogAction(a)}</div>
  </div>`;
}

function catalogHTML() {
  if (!state.catalog) {
    if (state.catalogError) return `<div class="empty"><h2>The catalog could not be loaded</h2><p>${esc(state.catalogError)}</p><button class="btn" data-act="reload-catalog">Try Again</button></div>`;
    return `<div class="empty"><span class="spinner"></span></div>`;
  }
  const shown = state.catalog.filter((a) =>
    (state.category === "all" || (a.category || "other") === state.category) && matches(catalogText(a)));
  if (!shown.length) {
    if (state.query) return noMatches();
    return `<div class="empty"><h2>Nothing in this category</h2></div>`;
  }
  // Rendering every row is fine for a few hundred agents, and keeps
  // scrolling and find-as-you-type simple.
  return `<div class="list">${shown.map(catalogRow).join("")}</div>`;
}

function agentHTML(id) {
  const def = state.byId.get(id);
  const mine = installsOf(id);
  if (!def && !mine.length) {
    if (!state.catalog && !state.catalogError) return `<div class="empty"><span class="spinner"></span></div>`;
    return `<div class="empty"><h2>Agent not found</h2><p>It may have been removed from the catalog.</p><button class="btn" data-go="/catalog">Browse Catalog</button></div>`;
  }
  const name = def ? def.name : mine[0].name;
  const meta = [];
  if (def && def.category) meta.push(`<span class="tag plain">${esc(def.category)}</span>`);
  if (def && def.vendor) meta.push(`<span class="tag plain">${esc(def.vendor)}</span>`);
  if (def && def.license) meta.push(`<span class="tag plain">${esc(def.license)}</span>`);
  const links = def ? [["homepage", "Website"], ["repository", "Source"], ["documentation", "Docs"]]
    .filter(([k]) => def[k]).map(([k, label]) => `<button class="btn" data-act="open-url" data-id="${esc(id)}" data-kind="${k}">${label}</button>`).join("") : "";

  let html = `<div class="hero">${avatar(name, id, true)}<div>
    <h1 class="selectable">${esc(name)}</h1>
    <p class="selectable">${esc(def ? def.description : "")}</p>
    ${meta.length ? `<div class="hero-meta">${meta.join("")}</div>` : ""}
    ${links ? `<div class="links">${links}</div>` : ""}
  </div></div>`;

  if (mine.length) {
    html += `<div class="section-title">On this computer</div>`;
    html += mine.map((i) => {
      const job = pendingJob(i.agent_id, i.method);
      const buttons = job ? working(job)
        : (i.has_update ? `<button class="btn primary" data-act="update" data-id="${esc(id)}" data-method="${esc(i.method)}">Update to ${esc(i.latest)}</button>` : upToDate(i)) +
          `<button class="btn danger" data-act="uninstall" data-id="${esc(id)}" data-method="${esc(i.method)}" data-name="${esc(name)}">Uninstall</button>`;
      return `<div class="card">
        <div class="card-head"><div class="grow"><div class="card-title">Installed with ${esc(i.method)}</div><div class="card-sub">${versionHTML(i)}</div></div><div class="row-side">${buttons}</div></div>
        <dl class="kv">
          ${i.path ? `<dt>Location</dt><dd class="mono selectable">${esc(i.path)}</dd>` : ""}
          ${ago(i.detected_at) ? `<dt>Detected</dt><dd>${esc(ago(i.detected_at))}</dd>` : ""}
        </dl>
      </div>`;
    }).join("");
  }

  if (def) {
    const installed = new Set(mine.map((i) => i.method));
    const others = def.methods.filter((m) => !installed.has(m.name));
    if (others.length) {
      html += `<div class="section-title">${mine.length ? "Other ways to install" : "Install"}</div><div class="list">`;
      html += others.map((m) => {
        const job = pendingJob(id, m.name);
        const action = job ? working(job)
          : m.available ? `<button class="btn${mine.length ? "" : " primary"}" data-act="install" data-id="${esc(id)}" data-method="${esc(m.name)}">Install</button>`
          : `<span class="tag plain">${esc(m.name)} is not installed</span>`;
        return `<div class="row"><div class="row-main"><div class="row-title"><span class="tag">${esc(m.name)}</span>${m.package ? `<span class="name mono">${esc(m.package)}</span>` : ""}</div>${m.command ? `<div class="row-desc mono selectable">${esc(m.command)}</div>` : ""}</div><div class="row-side">${action}</div></div>`;
      }).join("");
      html += `</div>`;
    } else if (!mine.length) {
      html += `<div class="empty"><h2>Not available on this platform</h2></div>`;
    }
  }

  const history = jobs().filter((j) => j.agent_id === id && j.status === "failed").slice(-1);
  if (history.length) {
    html += `<div class="section-title">Last problem</div><div class="card"><div class="card-title">${esc(jobTitle(history[0]))} failed</div><div class="job-error selectable">${esc(history[0].error)}</div></div>`;
  }
  return html;
}

function jobTitle(j) {
  const verb = { install: "Install", update: "Update", uninstall: "Uninstall" }[j.action] || j.action;
  return `${verb} ${j.agent_name}`;
}

function activityHTML() {
  const list = jobs().slice().reverse();
  if (!list.length) {
    return `<div class="empty"><h2>No activity yet</h2><p>Installs, updates and removals show up here with their results.</p></div>`;
  }
  return `<div class="list">${list.map((j) => {
    const icon = j.status === "running" ? '<span class="spinner"></span>' : ICONS[j.status];
    const note = j.status === "queued" ? "Waiting" : j.status === "running" ? "In progress…" : j.status === "done" ? (j.detail || "Done") : "Failed";
    return `<div class="row link" data-go="/agent/${encodeURIComponent(j.agent_id)}">
      <span class="job-icon ${j.status}">${icon}</span>
      <div class="row-main">
        <div class="row-title"><span class="name">${esc(jobTitle(j))}</span><span class="tag">${esc(j.method)}</span></div>
        <div class="row-desc">${esc(note)}</div>
        ${j.error ? `<div class="job-error selectable">${esc(j.error)}</div>` : ""}
      </div>
      <span class="when">${esc(ago(j.finished_at || j.created_at))}</span>
    </div>`;
  }).join("")}</div>`;
}

function toggle(key, on) {
  return `<button class="switch" role="switch" aria-checked="${on ? "true" : "false"}" data-setting="${key}" aria-label="${key}"></button>`;
}

function settingsHTML() {
  const s = state.data.settings;
  let cliSide = "";
  if (s.cli_installed) {
    cliSide = (s.cli_can_install ? `<button class="btn" data-act="cli" data-do="install">Reinstall</button>` : "") +
      (s.cli_can_uninstall ? `<button class="btn danger" data-act="cli" data-do="uninstall">Remove</button>` : "") ||
      `<span class="tag ok">Installed</span>`;
  } else if (s.cli_can_install) {
    cliSide = `<button class="btn primary" data-act="cli" data-do="install">Install</button>`;
  }
  const cliDesc = s.cli_installed
    ? `<div class="setting-desc mono selectable">${esc(s.cli_path)}</div>`
    : `<div class="setting-desc">${s.cli_can_install ? "Install <span class=\"mono\">agentmgr</span> to manage agents from a terminal." : "Not installed. This build does not include it."}</div>`;
  const cli = `<div class="setting"><div class="grow"><div class="setting-title">Command line tool</div>${cliDesc}</div><div class="row-side">${cliSide}</div></div>`;

  return `<div class="section-title">Updates</div>
  <div class="list">
    <div class="setting"><div class="grow"><div class="setting-title">Check for updates automatically</div><div class="setting-desc">Look for new versions of your installed agents in the background.</div></div>${toggle("auto_check", s.auto_check)}</div>
    <div class="setting"><div class="grow"><div class="setting-title">Notify me about updates</div><div class="setting-desc">Show a notification when new versions are found.</div></div>${toggle("notify", s.notify)}</div>
  </div>
  <div class="section-title">General</div>
  <div class="list">
    <div class="setting"><div class="grow"><div class="setting-title">Start at login</div><div class="setting-desc">Keep AgentManager running in the background after you sign in.</div></div>${toggle("auto_start", s.auto_start)}</div>
    ${cli}
    <div class="setting"><div class="grow"><div class="setting-title">Terminal interface</div><div class="setting-desc">Open the full-screen terminal version of AgentManager.</div></div><button class="btn" data-act="open-tui"${s.tui_available ? "" : " disabled"}>Open in Terminal</button></div>
  </div>
  <div class="section-title">About</div>
  <div class="card"><dl class="kv">
    <dt>Version</dt><dd class="selectable">${esc(state.data.version)}</dd>
    <dt>Settings file</dt><dd class="mono selectable">${esc(s.config_path)}</dd>
  </dl></div>`;
}

// ---------- events ----------

document.addEventListener("click", (e) => {
  const act = e.target.closest("[data-act]");
  if (act) {
    e.stopPropagation();
    handleAction(act);
    return;
  }
  const cat = e.target.closest("[data-cat]");
  if (cat) {
    state.category = cat.dataset.cat;
    render();
    return;
  }
  const sw = e.target.closest("[data-setting]");
  if (sw) {
    const on = sw.getAttribute("aria-checked") !== "true";
    sw.setAttribute("aria-checked", String(on));
    post("/api/settings", { [sw.dataset.setting]: on });
    return;
  }
  if (e.target.closest("select, input")) return;
  const nav = e.target.closest("[data-go]");
  if (nav) go(nav.dataset.go);
});

document.addEventListener("change", (e) => {
  const sel = e.target.closest("[data-method-for]");
  if (sel) state.methodChoice[sel.dataset.methodFor] = sel.value;
});

document.addEventListener("input", (e) => {
  if (e.target.id === "search") {
    state.query = e.target.value;
    render();
  }
});

document.addEventListener("keydown", (e) => {
  const search = $("#search");
  if ((e.metaKey || e.ctrlKey) && e.key === "f" && search) {
    e.preventDefault();
    search.focus();
    search.select();
  } else if (e.key === "Escape") {
    if (search && state.query) {
      state.query = "";
      search.value = "";
      render();
    } else if (state.route.view === "agent") {
      history.back();
    }
  }
});

function handleAction(el) {
  const d = el.dataset;
  switch (d.act) {
    case "back":
      if (history.length > 1) history.back(); else go("/installed");
      break;
    case "refresh":
      post("/api/refresh");
      break;
    case "update-all":
      post("/api/update-all").then((out) => { if (out) go("/activity"); });
      break;
    case "clear-jobs":
      post("/api/jobs/clear");
      break;
    case "clear-search":
      state.query = "";
      if ($("#search")) $("#search").value = "";
      render();
      break;
    case "reload-catalog":
      loadCatalog();
      break;
    case "install": {
      const def = state.byId.get(d.id);
      const method = d.method || (def ? chosenMethod(def) : "");
      post("/api/action", { agent_id: d.id, method, action: "install" });
      break;
    }
    case "update":
      post("/api/action", { agent_id: d.id, method: d.method, action: "update" });
      break;
    case "uninstall":
      confirmDialog(`Uninstall ${d.name}?`, `This removes the ${d.method} installation from this computer.`, "Uninstall")
        .then((yes) => { if (yes) post("/api/action", { agent_id: d.id, method: d.method, action: "uninstall" }); });
      break;
    case "open-url":
      post("/api/open-url", { agent_id: d.id, kind: d.kind });
      break;
    case "open-tui":
      post("/api/open-tui");
      break;
    case "cli":
      el.disabled = true;
      post("/api/cli", { action: d.do }, d.do === "install" ? "Command line tool installed" : "Command line tool removed")
        .finally(() => { el.disabled = false; });
      break;
  }
}

// ---------- start ----------

state.route = parseRoute();
render();
loadCatalog();
poll();
