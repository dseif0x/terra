// Map styles. "Satellite" is Esri World Imagery with the labels, roads and
// borders of OpenFreeMap's Liberty style on top (restyled to read on
// imagery); "Map" is Liberty itself. Both get mountain peaks (which Liberty
// leaves out), a terrain source for 3D, and the globe.

const LIBERTY = 'https://tiles.openfreemap.org/styles/liberty';
const ESRI = 'https://server.arcgisonline.com/ArcGIS/rest/services/World_Imagery/MapServer/tile/{z}/{y}/{x}';
const ESRI_ATTRIBUTION =
  'Imagery © <a href="https://www.esri.com/" target="_blank" rel="noopener">Esri</a>, Maxar, Earthstar Geographics, and the GIS User Community';
const TERRAIN = 'https://s3.amazonaws.com/elevation-tiles-prod/terrarium/{z}/{x}/{y}.png';

let base;
export async function loadBase() {
  if (!base) {
    const r = await fetch(LIBERTY);
    if (!r.ok) throw new Error('map style: ' + r.status);
    base = await r.json();
  }
  return base;
}

// Layers kept on the satellite view: lines worth seeing from above, and
// every label.
const KEEP_LINES = /^(road|bridge)_(motorway|trunk_primary|secondary_tertiary|minor|link)$|^boundary_|^waterway_river$|^aeroway_runway$/;
const DROP_SYMBOLS = /^road_one_way|^road_shield_us|^highway-shield-us/;

// Source layers whose labels open a place when clicked. "poi" means the
// backend looks for that name close by, "place" further out.
export const CLICKABLE = {
  poi: 'poi',
  mountain_peak: 'poi',
  aerodrome_label: 'poi',
  place: 'place',
  water_name: 'place',
};

function peakLayer(onImagery) {
  return {
    id: 'terra_peaks',
    type: 'symbol',
    source: 'openmaptiles',
    'source-layer': 'mountain_peak',
    minzoom: 7,
    filter: ['all', ['has', 'name'], ['==', ['get', 'class'], 'peak']],
    layout: {
      'text-field': [
        'concat',
        '▲ ',
        ['coalesce', ['get', 'name:en'], ['get', 'name:latin'], ['get', 'name']],
        ['case', ['has', 'ele'], ['concat', '\n', ['to-string', ['get', 'ele']], ' m'], ''],
      ],
      'text-font': ['Noto Sans Regular'],
      'text-size': 11,
      'text-anchor': 'top',
      'text-offset': [0, -0.4],
      'symbol-sort-key': ['coalesce', ['get', 'rank'], 99],
    },
    paint: onImagery
      ? { 'text-color': '#fff', 'text-halo-color': 'rgba(0,0,0,0.8)', 'text-halo-width': 1.4 }
      : { 'text-color': '#6b4f2a', 'text-halo-color': 'rgba(255,255,255,0.9)', 'text-halo-width': 1.2 },
  };
}

// English names only (Liberty shows "Wien" and native scripts next to the
// Latin name): every label that shows a name gets English, else Latin, else
// the local name.
const NAME = ['coalesce', ['get', 'name:en'], ['get', 'name:latin'], ['get', 'name']];
function english(field) {
  return JSON.stringify(field).includes('"name') ? NAME : field;
}

// Points of interest from lower zooms than Liberty's 15/16/17: rank is per
// tile, so in a remote area the few POIs there are rank 1 and show early,
// while dense cities still only show their top-ranked ones.
const POI_MINZOOM = { poi_r1: 12, poi_r7: 14, poi_r20: 15 };

function extras(style, onImagery) {
  style.sources.terrain = {
    type: 'raster-dem',
    tiles: [TERRAIN],
    encoding: 'terrarium',
    tileSize: 256,
    maxzoom: 15,
    attribution: 'Terrain: <a href="https://registry.opendata.aws/terrain-tiles/" target="_blank" rel="noopener">Mapzen/AWS</a>',
  };
  style.projection = { type: 'globe' };
  style.sky = {
    'sky-color': '#1d4f91',
    'horizon-color': '#a7c8ec',
    'fog-color': '#cfe0f5',
    'sky-horizon-blend': 0.6,
    'horizon-fog-blend': 0.6,
    'fog-ground-blend': 0.8,
    'atmosphere-blend': ['interpolate', ['linear'], ['zoom'], 0, 1, 6, 0.6, 10, 0],
  };
  for (const l of style.layers) {
    if (l.type === 'symbol' && l.layout?.['text-field']) l.layout['text-field'] = english(l.layout['text-field']);
    if (POI_MINZOOM[l.id] !== undefined) l.minzoom = POI_MINZOOM[l.id];
  }
  style.layers.push(peakLayer(onImagery));
  return style;
}

export function mapStyle(b) {
  return extras(structuredClone(b), false);
}

export function satelliteStyle(b) {
  const s = structuredClone(b);
  s.sources = {
    satellite: { type: 'raster', tiles: [ESRI], tileSize: 256, maxzoom: 19, attribution: ESRI_ATTRIBUTION },
    openmaptiles: s.sources.openmaptiles,
  };
  const layers = [{ id: 'satellite', type: 'raster', source: 'satellite' }];
  for (const l of s.layers) {
    if (l.source !== 'openmaptiles') continue;
    if (l.type === 'line' && KEEP_LINES.test(l.id)) {
      const major = /motorway|trunk_primary/.test(l.id);
      l.paint = {
        ...l.paint,
        'line-color': l.id.startsWith('boundary')
          ? 'rgba(255,255,255,0.7)'
          : l.id.startsWith('waterway')
            ? 'rgba(140,200,255,0.35)'
            : major
              ? 'rgba(255,214,120,0.6)'
              : 'rgba(255,255,255,0.32)',
      };
      if (!l.id.startsWith('boundary')) l.minzoom = Math.max(l.minzoom ?? 0, major ? 6 : 13);
      delete l.paint['line-gap-width'];
      layers.push(l);
    } else if (l.type === 'symbol' && !DROP_SYMBOLS.test(l.id)) {
      if (l.layout?.['text-field']) {
        const water = l['source-layer'] === 'water_name' || l['source-layer'] === 'waterway';
        l.paint = {
          ...l.paint,
          'text-color': water ? '#cfe8ff' : '#ffffff',
          'text-halo-color': 'rgba(0,0,0,0.78)',
          'text-halo-width': 1.4,
          'text-halo-blur': 0.3,
        };
      }
      layers.push(l);
    }
  }
  s.layers = layers;
  return extras(s, true);
}

// The layers that hold labels and overlays (everything but imagery), for
// the labels toggle.
export function overlayLayers(map) {
  return map
    .getStyle()
    .layers.filter((l) => l.id !== 'satellite' && l.source === 'openmaptiles' && l.type !== 'fill')
    .map((l) => l.id);
}
