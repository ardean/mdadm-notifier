async function fetchJSON(url, options = {}) {
  const response = await fetch(url, { cache: "no-store", ...options });
  if (!response.ok) {
    throw new Error(`HTTP ${response.status}`);
  }
  return response.json();
}

export function getStatus() {
  return fetchJSON("/api/status");
}

export function refreshStatus() {
  return fetchJSON("/api/refresh", { method: "POST" });
}
