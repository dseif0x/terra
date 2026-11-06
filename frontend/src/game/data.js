// Country data for the game (built by scripts/game-data.mjs). Countries
// and the other areas on the map share one index space: features[i] is
// countries[i] for i < countries.length, then the others.
import { mainParts, boundsOf } from './geo.js';

let loading;
export function loadData() {
  loading ??= fetch('/game/countries.json').then((r) => {
    if (!r.ok) throw new Error('game data: ' + r.status);
    return r.json().then(prepare);
  });
  return loading;
}

function prepare(d) {
  const n = d.countries.length;
  d.countries.forEach((c, i) => (c.idx = i));
  d.others.forEach((o, i) => (o.idx = n + i));
  const byId = new Map(d.countries.map((c) => [c.id, c]));
  const feature = (idx) => d.features.features[idx];
  const frames = new Map();
  return {
    ...d,
    byId,
    feature,
    areaAt: (idx) => (idx == null ? null : idx < n ? d.countries[idx] : d.others[idx - n]),
    // where the camera goes to show a country
    frameOf(c) {
      if (!frames.has(c.idx)) frames.set(c.idx, boundsOf(mainParts(feature(c.idx))));
      return frames.get(c.idx);
    },
  };
}

export const REGIONS = [
  { key: 'all', label: 'World' },
  { key: 'Africa', label: 'Africa' },
  { key: 'Americas', label: 'Americas' },
  { key: 'Caribbean', label: 'Caribbean' },
  { key: 'Asia', label: 'Asia' },
  { key: 'Europe', label: 'Europe' },
  { key: 'Oceania', label: 'Oceania' },
];
export const NEW_REGION = { key: 'new', label: 'Since 1990' };

export const regionLabel = (key) => [...REGIONS, NEW_REGION].find((r) => r.key === key)?.label ?? key;

export function inRegion(c, region) {
  if (region === 'all') return true;
  if (region === 'new') return c.isNew;
  if (region === 'Caribbean') return c.subregion === 'Caribbean';
  return c.region === region;
}

export const poolFor = (data, region = 'all', diff = 'all') =>
  data.countries.filter((c) => inRegion(c, region) && (diff === 'all' || c.diff === diff));

// Where the globe turns to when a region is picked: centre and zoom.
export const REGION_VIEW = {
  all: [15, 20, 1.5],
  Africa: [18, 2, 2.1],
  Americas: [-78, 8, 1.4],
  Caribbean: [-71, 17, 3.9],
  Asia: [88, 30, 1.9],
  Europe: [14, 52, 2.9],
  Oceania: [158, -14, 2.1],
  new: [45, 45, 2.2],
};

export const flagUrl = (c) => `/game/flags/${c.iso2}.svg`;
export const fmt = (n) => n.toLocaleString('en-US');
