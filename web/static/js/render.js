import {
  escapeHtml,
  pill,
  formatTime,
  formatDuration,
  formatBlocks,
  formatSpeed,
  formatAction,
  formatTemperature,
} from "./utils.js";

function renderMonitored(rows) {
  if (!rows || rows.length === 0) {
    return `<p class="empty">No monitored counters reported.</p>`;
  }

  const body = rows.map((row) => {
    const classes = [];
    if (row.alert) classes.push("alert");
    if (row.increased) classes.push("increased");
    const previous = row.previous == null ? "—" : row.previous;
    const delta = row.increased ? pill("increased", "warn") : "";
    const alert = row.alert ? pill("alert", "bad") : "";
    return `<tr class="${classes.join(" ")}">
      <td>${escapeHtml(row.name)}</td>
      <td>${escapeHtml(row.value)}</td>
      <td>${escapeHtml(previous)}</td>
      <td>${escapeHtml(row.threshold)}</td>
      <td>${delta}${alert}</td>
    </tr>`;
  }).join("");

  return `<table>
    <thead><tr><th>Counter</th><th>Value</th><th>Previous</th><th>Threshold</th><th>Flags</th></tr></thead>
    <tbody>${body}</tbody>
  </table>`;
}

function renderAttributes(rows) {
  if (!rows || rows.length === 0) {
    return `<p class="empty">No SMART attributes parsed.</p>`;
  }

  const body = rows.map((row) => {
    const flag = row.fail_flag && row.fail_flag !== "-" ? pill(row.fail_flag, "bad") : "";
    return `<tr>
      <td>${escapeHtml(row.id)}</td>
      <td>${escapeHtml(row.name)}</td>
      <td>${escapeHtml(row.raw_text || row.raw)}</td>
      <td>${flag}</td>
    </tr>`;
  }).join("");

  return `<table>
    <thead><tr><th>ID</th><th>Name</th><th>Raw</th><th>Flag</th></tr></thead>
    <tbody>${body}</tbody>
  </table>`;
}

function renderSelfTest(selfTest) {
  if (!selfTest) {
    return `<p class="empty">Self-test data not collected.</p>`;
  }

  const lines = [];
  lines.push(`<dl class="kv">
    <dt>Power-on hours</dt><dd>${escapeHtml(selfTest.power_on_hours || "—")}</dd>
    <dt>In progress</dt><dd>${selfTest.in_progress ? pill("yes", "warn") : pill("no", "ok")}</dd>
    <dt>Short due</dt><dd>${selfTest.short_due ? pill("yes", "warn") : pill("no", "ok")}</dd>
    <dt>Long due</dt><dd>${selfTest.long_due ? pill("yes", "warn") : pill("no", "ok")}</dd>
  </dl>`);

  if (selfTest.latest_short) {
    const entry = selfTest.latest_short;
    lines.push(`<p><strong>Latest short:</strong> ${escapeHtml(entry.description)} — ${escapeHtml(entry.status)}</p>`);
  }
  if (selfTest.latest_long) {
    const entry = selfTest.latest_long;
    lines.push(`<p><strong>Latest long:</strong> ${escapeHtml(entry.description)} — ${escapeHtml(entry.status)}</p>`);
  }

  if (selfTest.recent_entries && selfTest.recent_entries.length > 0) {
    const body = selfTest.recent_entries.map((entry) => {
      const statusKind = entry.in_progress ? "warn" : (entry.passed ? "ok" : "bad");
      return `<tr>
        <td>#${escapeHtml(entry.num)}</td>
        <td>${escapeHtml(entry.description)}</td>
        <td>${pill(entry.status, statusKind)}</td>
        <td>${escapeHtml(entry.lifetime_hours || "—")}</td>
      </tr>`;
    }).join("");

    lines.push(`<div class="section-gap"><table>
      <thead><tr><th>#</th><th>Test</th><th>Status</th><th>POH</th></tr></thead>
      <tbody>${body}</tbody>
    </table></div>`);
  }

  return lines.join("");
}

function renderSyncProgress(sync) {
  if (!sync || !sync.active) return "";

  const action = formatAction(sync.action);
  let label;
  let barWidth = 0;
  let barClass = "progress-bar";

  if (sync.pending) {
    label = `${action} pending`;
    barClass += " indeterminate";
  } else if (sync.delayed) {
    label = `${action} delayed`;
    barClass += " indeterminate";
  } else {
    const percent = Math.min(100, Math.max(0, sync.percent || 0));
    const hasBlockCounts = sync.completed_blocks != null && sync.total_blocks != null;
    if (percent === 0 && !hasBlockCounts) {
      label = `${action} in progress`;
      barClass += " indeterminate";
    } else {
      label = `${action} ${percent.toFixed(1)}%`;
      barWidth = percent;
    }
  }

  const meta = [];
  if (!sync.pending && !sync.delayed) {
    if (sync.completed_blocks != null && sync.total_blocks != null) {
      meta.push(`<span>${formatBlocks(sync.completed_blocks)} / ${formatBlocks(sync.total_blocks)} blocks</span>`);
    }
    if (sync.finish_minutes != null) {
      meta.push(`<span>ETA ${formatDuration(sync.finish_minutes)}</span>`);
    }
    if (sync.speed_kbps) {
      meta.push(`<span>${formatSpeed(sync.speed_kbps)}</span>`);
    }
  }

  return `<div class="section-gap" id="sync-progress">
    <p class="progress-label">${escapeHtml(label)}</p>
    <div class="progress" role="progressbar" aria-valuemin="0" aria-valuemax="100" aria-valuenow="${barWidth}">
      <div class="${barClass}" style="width: ${barWidth}%"></div>
    </div>
    ${meta.length ? `<div class="progress-meta">${meta.join("")}</div>` : ""}
  </div>`;
}

export function syncProgressHTML(sync) {
  return renderSyncProgress(sync);
}

function raidStatusPill(raid) {
  if (raid.error) return pill("check failed", "bad");
  if (raid.sync?.active) {
    const action = raid.sync.action || "sync";
    if (raid.sync.pending) return pill(`${action} pending`, "warn");
    if (raid.sync.delayed) return pill(`${action} delayed`, "warn");
    return pill(`${action} ${(raid.sync.percent || 0).toFixed(1)}%`, "warn");
  }
  return pill(raid.healthy ? "healthy" : "degraded", raid.healthy ? "ok" : "bad");
}

function renderDisk(disk) {
  const statusKind = !disk.read_ok ? "bad" : (disk.healthy ? "ok" : "bad");
  const statusText = !disk.read_ok ? "read failed" : (disk.healthy ? "healthy" : "issues");

  let issues = "";
  if (disk.issues && disk.issues.length > 0) {
    issues = `<ul class="issues">${disk.issues.map((item) => `<li>${escapeHtml(item)}</li>`).join("")}</ul>`;
  }

  let warnings = "";
  if (disk.nvme_warnings && disk.nvme_warnings.length > 0) {
    warnings = `<ul class="issues">${disk.nvme_warnings.map((item) => `<li>${escapeHtml(item)}</li>`).join("")}</ul>`;
  }

  return `<section class="card">
    <div class="disk-header">
      <h2 class="disk-title">${escapeHtml(disk.device)}</h2>
      ${pill(statusText, statusKind)}
    </div>
    <dl class="kv">
      <dt>Manufacturer</dt><dd>${escapeHtml(disk.manufacturer || "—")}</dd>
      <dt>Model</dt><dd>${escapeHtml(disk.model || "—")}</dd>
      <dt>Capacity</dt><dd>${escapeHtml(disk.capacity || "—")}</dd>
      <dt>Temperature</dt><dd>${formatTemperature(disk.temperature_c)}</dd>
      <dt>Serial</dt><dd>${escapeHtml(disk.serial || "—")}</dd>
      <dt>SMART status</dt><dd>${escapeHtml(disk.health_status || "—")}</dd>
      <dt>Marginal warning</dt><dd>${disk.marginal_warning ? pill("yes", "warn") : pill("no", "ok")}</dd>
      <dt>Device errors</dt><dd>${escapeHtml(disk.device_errors ?? "—")}</dd>
    </dl>
    ${issues}
    ${warnings}
    <h3 class="section-gap">Monitored counters</h3>
    ${renderMonitored(disk.monitored)}
    <details>
      <summary>Self-tests</summary>
      ${renderSelfTest(disk.self_test)}
    </details>
    <details>
      <summary>All SMART attributes (${(disk.attributes || []).length})</summary>
      ${renderAttributes(disk.attributes)}
    </details>
  </section>`;
}

export function renderStatus(elements, data) {
  const { title, subtitle, overallBadge, content } = elements;

  title.textContent = data.hostname || "mdadm-notifier";
  subtitle.textContent = `${data.md_device || "—"} · last check ${formatTime(data.checked_at)} · health checks every ${data.config?.check_interval || "—"}`;

  if (!data.checked_at) {
    overallBadge.className = "badge neutral";
    overallBadge.textContent = "Waiting";
    content.innerHTML = `<section class="card"><p class="empty">Waiting for the first health check…</p></section>`;
    return;
  }

  const raid = data.raid || {};
  const syncActive = !!raid.sync?.active;
  overallBadge.className = "badge " + (data.healthy ? "ok" : (syncActive ? "warn" : "bad"));
  overallBadge.textContent = data.healthy ? "All healthy" : (syncActive ? "Rebuild in progress" : "Issues detected");

  let raidIssues = "";
  if (raid.error) {
    raidIssues = `<p class="issues">${escapeHtml(raid.error)}</p>`;
  } else if (raid.issues && raid.issues.length > 0) {
    raidIssues = `<ul class="issues">${raid.issues.map((item) => `<li>${escapeHtml(item)}</li>`).join("")}</ul>`;
  }

  const raidCard = `<section class="card" id="raid-card">
    <div class="disk-header">
      <h2 class="disk-title">RAID ${escapeHtml(raid.device || data.md_device || "—")}</h2>
      <span id="raid-status-pill">${raidStatusPill(raid)}</span>
    </div>
    <dl class="kv">
      <dt>Member disks</dt><dd>${escapeHtml((raid.devices || []).join(", ") || "—")}</dd>
      <dt>Self-tests</dt><dd>${data.config?.self_test_enabled ? "enabled" : "disabled"}</dd>
    </dl>
    ${renderSyncProgress(raid.sync)}
    ${raidIssues}
    <details>
      <summary>mdadm detail</summary>
      <pre>${escapeHtml(raid.detail || "No detail available.")}</pre>
    </details>
  </section>`;

  const disks = (data.disks || []).map(renderDisk).join("");
  content.innerHTML = `${raidCard}${disks ? `<div class="grid two section-gap">${disks}</div>` : `<section class="card section-gap"><p class="empty">No member disks found.</p></section>`}`;
}

export function updateSyncProgress(elements, sync, data) {
  if (!data) return;

  data.raid = data.raid || {};
  data.raid.sync = sync;

  const raid = data.raid;
  const syncActive = !!sync?.active;

  const { overallBadge } = elements;
  overallBadge.className = "badge " + (data.healthy ? "ok" : (syncActive ? "warn" : "bad"));
  overallBadge.textContent = data.healthy ? "All healthy" : (syncActive ? "Rebuild in progress" : "Issues detected");

  const pillEl = document.getElementById("raid-status-pill");
  if (pillEl) {
    pillEl.innerHTML = raidStatusPill(raid);
  }

  const raidCard = document.getElementById("raid-card");
  if (!raidCard) return;

  let syncEl = document.getElementById("sync-progress");
  const html = syncProgressHTML(sync);

  if (html) {
    if (syncEl) {
      syncEl.outerHTML = html;
    } else {
      const kv = raidCard.querySelector(".kv");
      if (kv) {
        kv.insertAdjacentHTML("afterend", html);
      }
    }
  } else if (syncEl) {
    syncEl.remove();
  }
}
