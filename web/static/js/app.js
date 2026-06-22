import { getStatus, refreshStatus } from "./api.js";
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

async function refresh(force = false) {
  if (refreshing) return;
  refreshing = true;
  refreshButton.disabled = true;
  refreshButton.textContent = force ? "Checking…" : "Refreshing…";

  try {
    const data = force ? await refreshStatus() : await getStatus();
    currentData = data;
    renderStatus(elements, data);
    manageSyncConnection();
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

refreshButton.addEventListener("click", () => refresh(true));
refresh(false);
setInterval(() => refresh(false), 30000);
