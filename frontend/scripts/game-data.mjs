// Builds the country data for Game mode from open data, at build time:
//   world-atlas (Natural Earth 1:50m, public domain) for the outlines,
//   world-countries (mledoze, ODbL) for names, capitals, regions and areas,
//   flag-icons (MIT) for the flags.
// Writes public/game/countries.json and public/game/flags/*.svg (both
// generated, not in git).
import { readFileSync, writeFileSync, mkdirSync, copyFileSync, rmSync } from 'node:fs';
import { createRequire } from 'node:module';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { feature, merge } from 'topojson-client';

const require = createRequire(import.meta.url);
const here = dirname(fileURLToPath(import.meta.url));
const out = join(here, '..', 'public', 'game');
const atlas = (n) => require(`world-atlas/countries-${n}.json`);
const wc = require('world-countries/countries.json');

// The 193 UN members plus the Vatican, Palestine, Kosovo and Taiwan.
const EXTRA = new Set(['VAT', 'PSE', 'UNK', 'TWN']);
const targets = wc.filter((c) => c.unMember || EXTRA.has(c.cca3));

// Countries most people place without thinking (the rest is decided by
// size and region below).
const EASY = new Set(
  ('USA CAN MEX BRA ARG CHL PER COL VEN CUB GBR IRL FRA ESP PRT ITA DEU NOR SWE FIN ISL POL UKR GRC TUR RUS ' +
    'CHN JPN KOR IND PAK IRN IRQ SAU AFG MNG KAZ THA VNM IDN PHL AUS NZL EGY LBY DZA MAR ZAF NGA ETH ' +
    'COD MDG KEN SOM SDN ISR CHE NLD BEL AUT').split(' '),
);
const HARD = new Set(
  ('GNQ GNB BDI RWA LSO SWZ DJI TGO BEN GMB CPV STP COM SYC MUS BFA CAF COG GAB SLE LBR GIN ERI SSD MWI ' +
    'TKM KGZ TJK BTN BRN TLS ARM AZE GEO MDA MKD MNE UNK LIE AND SMR MCO VAT MLT LUX BHR PSE ' +
    'ATG BRB DMA GRD KNA LCA VCT TTO BLZ SLV GUY SUR SLB VUT WSM TON KIR TUV NRU MHL FSM PLW MDV').split(' '),
);
// Independent since 1990.
const NEW = new Set(
  ('NAM YEM EST LVA LTU BLR UKR MDA GEO ARM AZE KAZ UZB TKM KGZ TJK SVN HRV BIH MKD SRB MNE UNK XKX CZE SVK ERI PLW MHL TLS SSD').split(' '),
);
const diffOf = (c) => {
  const k = c.cca3;
  if (EASY.has(k)) return 'easy';
  if (HARD.has(k) || c.area < 30000) return 'hard';
  return 'medium';
};

// Natural Earth parts that belong to a country here.
const ATTACH = { Somaliland: '706', 'N. Cyprus': '196' };

const t50 = atlas('50m');
const t10 = atlas('10m');
const geoms = t50.objects.countries.geometries;
const byId = new Map();
for (const g of geoms) {
  const id = g.id || (g.properties.name === 'Kosovo' ? 'UNK' : ATTACH[g.properties.name]);
  if (!id) continue;
  if (!byId.has(id)) byId.set(id, []);
  byId.get(id).push(g);
}

const round = (x) => Math.round(x * 1000) / 1000;
const roundCoords = (c) => (typeof c[0] === 'number' ? [round(c[0]), round(c[1])] : c.map(roundCoords));

// Natural Earth via topojson is spherical: Russia's and Fiji's rings jump
// across the antimeridian and Antarctica's ring circles the pole, which a
// flat GeoJSON renderer draws as lines across the world. Unwrap each ring
// so its longitudes are continuous (past 180 where needed), and close a
// ring that goes once around the globe through the pole.
function planar(geometry) {
  const polys = geometry.type === 'Polygon' ? [geometry.coordinates] : geometry.coordinates;
  const fixed = polys.map((poly) =>
    poly.map((ring) => {
      const out = [ring[0].slice()];
      let shift = 0;
      for (let i = 1; i < ring.length; i++) {
        const d = ring[i][0] - ring[i - 1][0];
        if (d > 180) shift -= 360;
        else if (d < -180) shift += 360;
        out.push([ring[i][0] + shift, ring[i][1]]);
      }
      if (shift !== 0) {
        const last = out.pop();
        const pole = last[1] < 0 ? -90 : 90;
        out.push(last, [last[0], pole], [out[0][0], pole], out[0].slice());
      }
      return out;
    }),
  );
  return geometry.type === 'Polygon' ? { type: 'Polygon', coordinates: fixed[0] } : { type: 'MultiPolygon', coordinates: fixed };
}
const geometryFor = (topo, list) =>
  list.length === 1 ? feature(topo, list[0]).geometry : merge(topo, list);

const NAMES = { CZE: "Czechia", MKD: "North Macedonia", COD: "DR Congo", COG: "Republic of the Congo", VAT: "Vatican City", UNK: "Kosovo", PSE: "Palestine", CIV: "Ivory Coast", FSM: "Micronesia", TLS: "Timor-Leste", SWZ: "Eswatini", MMR: "Myanmar", TUR: "Türkiye" };
const countries = [];
const features = [];
const seen = new Set();
for (const c of targets) {
  const key = c.ccn3 || c.cca3; // Kosovo has no numeric code
  let geometry;
  if (byId.has(key)) geometry = geometryFor(t50, byId.get(key));
  else {
    const g10 = t10.objects.countries.geometries.filter((g) => g.id === key);
    if (!g10.length) throw new Error('no outline for ' + c.name.common);
    geometry = geometryFor(t10, g10);
  }
  seen.add(key);
  const iso2 = c.cca2.toLowerCase();
  copyFlag(iso2);
  const i = countries.length;
  countries.push({
    id: c.cca3,
    iso2,
    name: NAMES[c.cca3] || c.name.common,
    official: c.name.official,
    capital: c.capital?.[0] || '',
    region: c.region,
    subregion: c.subregion,
    area: Math.round(c.area),
    latlng: c.latlng,
    diff: diffOf(c),
    small: c.area < 5000,
    isNew: NEW.has(c.cca3),
  });
  features.push({ type: 'Feature', id: i, properties: { t: 1 }, geometry: planar({ ...geometry, coordinates: roundCoords(geometry.coordinates) }) });
}
// Everything else on the map (territories, Western Sahara, ...): tappable,
// named, never asked for.
const others = [];
for (const [id, list] of byId) {
  if (seen.has(id)) continue;
  const c = wc.find((x) => x.ccn3 === id);
  const name = c ? c.name.common : list[0].properties.name;
  const owner = c && !c.independent ? sovereignOf(c) : '';
  const geometry = geometryFor(t50, list);
  const i = countries.length + others.length;
  others.push({ id: c?.cca3 || id, name, owner });
  features.push({ type: 'Feature', id: i, properties: { t: 0 }, geometry: planar({ ...geometry, coordinates: roundCoords(geometry.coordinates) }) });
}

function sovereignOf(c) {
  // world-countries has no sovereign field; the currency/idd hints are
  // unreliable, so use a short table for the territories big enough to tap
  const T = {
    GRL: 'Denmark', FRO: 'Denmark', PRI: 'United States', GUM: 'United States', ASM: 'United States', VIR: 'United States',
    MNP: 'United States', NCL: 'France', PYF: 'France', GUF: 'France', ATF: 'France', SPM: 'France', WLF: 'France',
    BLM: 'France', MAF: 'France', FLK: 'United Kingdom', SGS: 'United Kingdom', BMU: 'United Kingdom', CYM: 'United Kingdom',
    TCA: 'United Kingdom', VGB: 'United Kingdom', AIA: 'United Kingdom', MSR: 'United Kingdom', SHN: 'United Kingdom',
    PCN: 'United Kingdom', IOT: 'United Kingdom', GGY: 'United Kingdom', JEY: 'United Kingdom', IMN: 'United Kingdom',
    ABW: 'Netherlands', CUW: 'Netherlands', SXM: 'Netherlands', HKG: 'China', MAC: 'China', NFK: 'Australia',
    HMD: 'Australia', ALA: 'Finland', COK: 'New Zealand', NIU: 'New Zealand', ESH: '',
  };
  return T[c.cca3] ?? '';
}

function copyFlag(iso2) {
  mkdirSync(join(out, 'flags'), { recursive: true });
  copyFileSync(require.resolve(`flag-icons/flags/4x3/${iso2}.svg`), join(out, 'flags', `${iso2}.svg`));
}

rmSync(join(out, 'countries.json'), { force: true });
mkdirSync(out, { recursive: true });
writeFileSync(
  join(out, 'countries.json'),
  JSON.stringify({ countries, others, features: { type: 'FeatureCollection', features } }),
);
console.log(`game data: ${countries.length} countries, ${others.length} other areas`);
