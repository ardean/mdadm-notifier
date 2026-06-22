import { getStatus, refreshStatus } from "./api.js";
import { renderStatus } from "./render.js";

const elements = {
  content: document.getElementById("content"),
  title: document.getElementById("title"),
  subtitle: document.getElementById("subtitle"),
  overallBadge: document.getElementById("overall-badge"),
};

const refreshButton = document.getElementById("refresh-button");
let refreshing = false;

async function refresh(force = false) {
  if (refreshing) return;
  refreshing = true;
  refreshButton.disabled = true;
  refreshButton.textContent = force ? "Checking…" : "Refreshing…";

  try {
    const data = force ? await refreshStatus() : await getStatus();
    renderStatus(elements, data);
  } catch (err) {
    elements.subtitle.textContent = `Failed to load status: ${err.message}`;
    elements.overallBadge.className = "badge bad";
    elements.overallBadge.textContent = "Unavailable";
  } finally {
    refreshing = false;
    refreshButton.disabled = false;
    refreshButton.textContent = "Refresh";
  }
}

refreshButton.addEventListener("click", () => refresh(true));
refresh(false);
setInterval(() => refresh(false), 30000);
