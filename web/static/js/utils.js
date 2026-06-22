export function escapeHtml(value) {
  return String(value ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;");
}

export function pill(text, kind) {
  return `<span class="pill ${kind}">${escapeHtml(text)}</span>`;
}

export function formatTime(iso) {
  if (!iso) return "never";
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return iso;
  return date.toLocaleString();
}

export function formatDuration(minutes) {
  if (minutes == null || Number.isNaN(minutes)) return "—";
  if (minutes < 1) return "< 1 min";
  if (minutes < 60) return `${minutes.toFixed(1)} min`;
  const hours = Math.floor(minutes / 60);
  const mins = Math.round(minutes % 60);
  return `${hours}h ${mins}m`;
}

export function formatBlocks(value) {
  if (value == null) return "—";
  return Number(value).toLocaleString();
}

export function formatSpeed(kbps) {
  if (!kbps) return "—";
  if (kbps >= 1024) return `${(kbps / 1024).toFixed(1)} MB/s`;
  return `${kbps.toLocaleString()} KB/s`;
}

export function formatAction(action) {
  if (!action) return "Sync";
  return action.charAt(0).toUpperCase() + action.slice(1);
}

export function formatTemperature(temp) {
  if (temp == null || Number.isNaN(temp)) return "—";
  let kind = "ok";
  if (temp >= 55) kind = "bad";
  else if (temp >= 45) kind = "warn";
  return pill(`${temp} °C`, kind);
}
