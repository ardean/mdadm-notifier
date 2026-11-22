import { getStatus, refreshStatus, startSelfTest, selfTestStatus } from "./api.js";
import { renderStatus, updateSyncProgress } from "./render.js";

const elements = {
  content: document.getElementById("content"),
  title: document.getElementById("title"),
  subtitle: document.getElementById("subtitle"),
  overallBadge: document.getElementById("overall-badge"),
};

const refreshButton = document.getElementById("refresh-button");
let refreshing = false;
let currentData = null;
let syncSocket = null;
let syncReconnectTimer = null;
const pendingSelfTests = new Map();
const selfTestErrors = new Map();
const selfTestWatches = new Map();

function selfTestState() {
  return { pending: pendingSelfTests, errors: selfTestErrors };
}

function entryKey(entry) {
  if (!entry) return "";
  return `${entry.num}|${entry.lifetime_hours}|${entry.status}`;
}

function showStatus() {
  if (!currentData) return;
  renderStatus(elements, currentData, selfTestState());
}

function syncWebSocketURL() {
  const proto = location.protocol === "https:" ? "wss:" : "ws:";
  return `${proto}//${location.host}/api/ws/sync`;
}

function disconnectSyncWS() {
  if (syncReconnectTimer) {
    clearTimeout(syncReconnectTimer);
    syncReconnectTimer = null;
  }
  if (syncSocket) {
    syncSocket.close();
    syncSocket = null;
  }
}

function scheduleSyncReconnect() {
  if (syncReconnectTimer || !currentData?.raid?.sync?.active) return;
  syncReconnectTimer = setTimeout(() => {
    syncReconnectTimer = null;
    connectSyncWS();
  }, 2000);
}

function connectSyncWS() {
  if (syncSocket || !currentData?.raid?.sync?.active) return;

  const socket = new WebSocket(syncWebSocketURL());
  syncSocket = socket;

  socket.addEventListener("message", (event) => {
    let payload;
    try {
      payload = JSON.parse(event.data);
    } catch {
      return;
    }

    updateSyncProgress(elements, payload.sync, currentData);

    if (!payload.sync?.active) {
      disconnectSyncWS();
      refresh(false);
    }
  });

  socket.addEventListener("close", () => {
    if (syncSocket === socket) {
      syncSocket = null;
      scheduleSyncReconnect();
    }
  });

  socket.addEventListener("error", () => {
    socket.close();
  });
}

function manageSyncConnection() {
  if (currentData?.raid?.sync?.active) {
    connectSyncWS();
  } else {
    disconnectSyncWS();
  }
}

async function refreshWhenIdle(force) {
  while (refreshing) {
    await new Promise((resolve) => setTimeout(resolve, 250));
  }
  await refresh(force);
}

async function refresh(force = false) {
  if (refreshing) return;
  refreshing = true;
  refreshButton.disabled = true;
  refreshButton.textContent = force ? "Checking…" : "Refreshing…";

  try {
    const data = force ? await refreshStatus() : await getStatus();
    currentData = data;
    showStatus();
    manageSyncConnection();
    watchRunningTests(data);
  } catch (err) {
    elements.subtitle.textContent = `Failed to load status: ${err.message}`;
    elements.overallBadge.className = "badge bad";
    elements.overallBadge.textContent = "Unavailable";
    disconnectSyncWS();
  } finally {
    refreshing = false;
    refreshButton.disabled = false;
    refreshButton.textContent = "Refresh";
  }
}

function watchSelfTest(device, type, beforeKey) {
  const existing = selfTestWatches.get(device);
  if (existing) clearInterval(existing.timer);

  const started = Date.now();
  const timer = setInterval(async () => {
    let status;
    try {
      status = await selfTestStatus(device);
    } catch {
      if (Date.now() - started > 24 * 60 * 60 * 1000) {
        clearInterval(timer);
        selfTestWatches.delete(device);
      }
      return;
    }

    const current = type === "long" ? status.latest_long : status.latest_short;
    const finished = status.in_progress
      ? false
      : (selfTestWatches.get(device)?.sawRunning || entryKey(current) !== beforeKey);

    if (status.in_progress) {
      const watch = selfTestWatches.get(device);
      if (watch) watch.sawRunning = true;
      return;
    }

    if (!finished) {
      if (Date.now() - started > 24 * 60 * 60 * 1000) {
        clearInterval(timer);
        selfTestWatches.delete(device);
        pendingSelfTests.delete(device);
        selfTestErrors.set(device, "The self-test is still running. Refresh later for the result.");
        showStatus();
      }
      return;
    }

    clearInterval(timer);
    selfTestWatches.delete(device);
    pendingSelfTests.delete(device);
    selfTestErrors.delete(device);
    await refreshWhenIdle(true);
  }, 15000);

  selfTestWatches.set(device, { timer, sawRunning: false });
}

function watchRunningTests(data) {
  for (const disk of data.disks || []) {
    if (!disk.self_test?.in_progress || selfTestWatches.has(disk.device) || pendingSelfTests.has(disk.device)) {
      continue;
    }
    const running = disk.self_test.latest_short?.in_progress
      ? "short"
      : (disk.self_test.latest_long?.in_progress ? "long" : "short");
    const before = running === "long" ? disk.self_test.latest_long : disk.self_test.latest_short;
    watchSelfTest(disk.device, running, entryKey(before));
  }
}

async function runSelfTest(device, type) {
  const disk = (currentData?.disks || []).find((item) => item.device === device);
  const before = type === "long" ? disk?.self_test?.latest_long : disk?.self_test?.latest_short;
  pendingSelfTests.set(device, type);
  selfTestErrors.delete(device);
  showStatus();

  try {
    await startSelfTest(device, type);
    watchSelfTest(device, type, entryKey(before));
  } catch (err) {
    pendingSelfTests.delete(device);
    selfTestErrors.set(device, err.message);
    showStatus();
  }
}

elements.content.addEventListener("click", (event) => {
  const button = event.target.closest("[data-selftest-type]");
  if (!button || button.disabled) return;

  const device = button.dataset.selftestDevice;
  const type = button.dataset.selftestType;
  if (!device || (type !== "short" && type !== "long")) return;
  if (type === "long" && !window.confirm(`Start a long self-test on ${device}? It reads the whole disk and can take many hours.`)) {
    return;
  }
  runSelfTest(device, type);
});

refreshButton.addEventListener("click", () => refresh(true));
refresh(false);
setInterval(() => refresh(false), 30000);
