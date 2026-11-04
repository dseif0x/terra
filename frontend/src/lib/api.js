async function getJSON(path, params, signal) {
  const q = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) if (v !== undefined && v !== null && v !== '') q.set(k, v);
  const r = await fetch(`${path}?${q}`, { signal });
  const body = await r.json().catch(() => null);
  if (!r.ok) throw new Error(body?.error || `${r.status} ${r.statusText}`);
  return body;
}

export const search = (q, center, zoom, signal) =>
  getJSON('/api/search', { q, lat: center?.lat.toFixed(4), lon: center?.lng.toFixed(4), zoom: zoom?.toFixed(0) }, signal);

export const place = (params, signal) => getJSON('/api/place', params, signal);

// "47.53, 13.6" or "47.53 13.6" -> {lat, lon}
export function parseCoords(s) {
  const m = s.trim().match(/^(-?\d{1,2}(?:\.\d+)?)\s*[,;\s]\s*(-?\d{1,3}(?:\.\d+)?)$/);
  if (!m) return null;
  const lat = +m[1];
  const lon = +m[2];
  return Math.abs(lat) <= 90 && Math.abs(lon) <= 180 ? { lat, lon } : null;
}
