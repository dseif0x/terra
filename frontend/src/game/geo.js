// Distances between a tap and a country's outline, and the direction to it.
import { geoContains } from 'd3-geo';

const R = 6371;
const rad = Math.PI / 180;

export function haversine(a, b) {
  const dLat = (b[1] - a[1]) * rad;
  const dLon = (b[0] - a[0]) * rad;
  const s = Math.sin(dLat / 2) ** 2 + Math.cos(a[1] * rad) * Math.cos(b[1] * rad) * Math.sin(dLon / 2) ** 2;
  return 2 * R * Math.asin(Math.min(1, Math.sqrt(s)));
}

const polygons = (g) => (g.type === 'Polygon' ? [g.coordinates] : g.coordinates);

// The point of a country's outline nearest to p ([lon, lat]), and how far
// it is in km (0 inside). Every edge is measured in a flat projection
// centred on the nearer end of the edge, which is exact enough for the
// short distances that decide a tap; far away, the nearest corner is used.
export function nearest(p, feature) {
  if (geoContains(feature, p)) return { km: 0, point: p };
  let best = Infinity;
  let point = null;
  for (const poly of polygons(feature.geometry)) {
    for (const ring of poly) {
      for (let i = 0; i < ring.length - 1; i++) {
        const a = ring[i];
        const d = haversine(p, a);
        if (d < best) {
          best = d;
          point = a;
        }
        if (d > best + 300) continue; // this edge cannot beat the best
        const r = edge(p, a, ring[i + 1]);
        if (r && r.km < best) {
          best = r.km;
          point = r.point;
        }
      }
    }
  }
  return { km: best, point };
}

function edge(p, a, b) {
  const k = Math.cos(a[1] * rad);
  const wrap = (d) => (d > 180 ? d - 360 : d < -180 ? d + 360 : d);
  const bx = wrap(b[0] - a[0]) * k;
  const by = b[1] - a[1];
  const px = wrap(p[0] - a[0]) * k;
  const py = p[1] - a[1];
  const len = bx * bx + by * by;
  if (!len) return null;
  const t = Math.max(0, Math.min(1, (px * bx + py * by) / len));
  const q = [a[0] + (wrap(b[0] - a[0]) * t), a[1] + by * t];
  return { km: haversine(p, q), point: q };
}

const ARROWS = ['↑', '↗', '→', '↘', '↓', '↙', '←', '↖'];
export function bearing(from, to) {
  const φ1 = from[1] * rad;
  const φ2 = to[1] * rad;
  const Δλ = (to[0] - from[0]) * rad;
  const y = Math.sin(Δλ) * Math.cos(φ2);
  const x = Math.cos(φ1) * Math.sin(φ2) - Math.sin(φ1) * Math.cos(φ2) * Math.cos(Δλ);
  return (Math.atan2(y, x) / rad + 360) % 360;
}
export const arrow = (deg) => ARROWS[Math.round(deg / 45) % 8];
const COMPASS = ['north', 'north-east', 'east', 'south-east', 'south', 'south-west', 'west', 'north-west'];
export const compassWord = (deg) => COMPASS[Math.round(deg / 45) % 8];

// The main landmass of a country and the parts close to it: what its
// silhouette shows and what the camera frames (France without French
// Guiana, the US without Hawaii's far islands still in view).
export function mainParts(feature, km = 1200) {
  const polys = polygons(feature.geometry);
  if (polys.length === 1) return polys;
  const areaOf = (ring) => {
    let s = 0;
    for (let i = 0; i < ring.length - 1; i++) s += ring[i][0] * ring[i + 1][1] - ring[i + 1][0] * ring[i][1];
    return Math.abs(s / 2) * Math.cos(ring[0][1] * rad);
  };
  const sized = polys.map((p) => ({ p, a: areaOf(p[0]), c: centre(p[0]) }));
  const big = sized.reduce((m, x) => (x.a > m.a ? x : m));
  return sized.filter((x) => x === big || haversine(x.c, big.c) < km + Math.sqrt(big.a) * 111).map((x) => x.p);
}

function centre(ring) {
  let x = 0;
  let y = 0;
  const ref = ring[0][0];
  for (const c of ring) {
    let d = c[0] - ref;
    if (d > 180) d -= 360;
    if (d < -180) d += 360;
    x += ref + d;
    y += c[1];
  }
  return [x / ring.length, y / ring.length];
}

// [[west, south], [east, north]] of some polygons, across the antimeridian
// when that is shorter (Fiji, Russia's far east).
export function boundsOf(polys) {
  let s = 90;
  let n = -90;
  const lons = [];
  for (const p of polys)
    for (const c of p[0]) {
      lons.push(c[0]);
      s = Math.min(s, c[1]);
      n = Math.max(n, c[1]);
    }
  let w = Math.min(...lons);
  let e = Math.max(...lons);
  if (e - w > 180) {
    const shifted = lons.map((x) => (x < 0 ? x + 360 : x));
    w = Math.min(...shifted);
    e = Math.max(...shifted);
  }
  return [
    [w, s],
    [e, n],
  ];
}
