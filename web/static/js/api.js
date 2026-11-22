async function fetchJSON(url, options = {}) {
  const response = await fetch(url, { cache: "no-store", ...options });
  const text = await response.text();
  let body = null;
  if (text) {
    try {
      body = JSON.parse(text);
    } catch {
      body = text;
    }
  }
  if (!response.ok) {
    const message = typeof body === "string" && body.trim() ? body.trim() : `HTTP ${response.status}`;
    throw new Error(message);
  }
  return body;
}

export function getStatus() {
  return fetchJSON("/api/status");
}

export function refreshStatus() {
  return fetchJSON("/api/refresh", { method: "POST" });
}

export function startSelfTest(device, type) {
  return fetchJSON("/api/selftest", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ device, type }),
  });
}

export function selfTestStatus(device) {
  const query = new URLSearchParams({ device });
  return fetchJSON(`/api/selftest?${query}`);
}
