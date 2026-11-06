// Seeded randomness and the daily calendar. The daily five depend only on
// the date, so everyone playing on the same day gets the same countries.

export function hashString(s) {
  let h = 2166136261 >>> 0; // FNV-1a
  for (let i = 0; i < s.length; i++) {
    h ^= s.charCodeAt(i);
    h = Math.imul(h, 16777619) >>> 0;
  }
  return h;
}

export function mulberry32(seed) {
  let a = seed >>> 0;
  return () => {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

export function shuffle(list, rand = Math.random) {
  const a = list.slice();
  for (let i = a.length - 1; i > 0; i--) {
    const j = Math.floor(rand() * (i + 1));
    [a[i], a[j]] = [a[j], a[i]];
  }
  return a;
}

// Days are local calendar days, keyed YYYY-MM-DD.
const pad = (n) => String(n).padStart(2, '0');
export const keyOf = (d) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
export const todayKey = () => keyOf(new Date());
export function dateOfKey(key) {
  const [y, m, d] = key.split('-').map(Number);
  return new Date(y, m - 1, d);
}
export function addDays(key, n) {
  const d = dateOfKey(key);
  d.setDate(d.getDate() + n);
  return keyOf(d);
}

// Daily #1 is the first day of 2026.
const EPOCH = '2026-01-01';
const DAY = 86400000;
export const dayNumber = (key) => Math.round((dateOfKey(key) - dateOfKey(EPOCH)) / DAY) + 1;
export const keyOfNumber = (n) => addDays(EPOCH, n - 1);
export const isValidDay = (key) => /^\d{4}-\d\d-\d\d$/.test(key) && dayNumber(key) >= 1 && key <= todayKey();

export function msUntilTomorrow() {
  const now = new Date();
  const next = new Date(now.getFullYear(), now.getMonth(), now.getDate() + 1);
  return next - now;
}

function tiers(countries, rand) {
  const t = { easy: [], medium: [], hard: [] };
  for (const c of countries) t[c.diff].push(c);
  return { easy: shuffle(t.easy, rand), medium: shuffle(t.medium, rand), hard: shuffle(t.hard, rand) };
}

// The daily five ramp up: two easy, two medium, one hard.
export function dailyCountries(countries, key) {
  const t = tiers(countries, mulberry32(hashString('terra:daily:' + key)));
  return [t.easy[0], t.easy[1], t.medium[0], t.medium[1], t.hard[0]];
}

// Blitz has its own five each day and a harder mix, in random order.
export function blitzCountries(countries, key) {
  const rand = mulberry32(hashString('terra:blitz:' + key));
  const t = tiers(countries, rand);
  return shuffle([t.easy[0], t.medium[0], t.medium[1], t.hard[0], t.hard[1]], rand);
}
