// Learn: spaced repetition. Every country has a level from 0 to 6; each
// level waits longer before the country comes back. A clean find moves it
// up, a miss sends it down two. What you know is kept per country, so it
// carries across regions, and the daily, Blitz and practice rounds count
// too.
import { state, save } from './store.js';

const MIN = 60000;
const HOUR = 60 * MIN;
const DAY = 24 * HOUR;
export const MAX_LEVEL = 6;
const WAIT = [2 * MIN, 10 * MIN, HOUR, DAY, 3 * DAY, 8 * DAY, 21 * DAY];

export const LEVEL_COLORS = ['#e5484d', '#ec6a4f', '#f0904a', '#e9b949', '#b5cf4c', '#5bc77d', '#2fb8a4'];
export const category = (level) =>
  level < 0 ? 'new' : level <= 2 ? 'learning' : level <= 4 ? 'known' : 'mastered';

const rec = (id) => state.knowledge[id] || null;
export const levelOf = (id) => rec(id)?.b ?? -1;

// Grades a finished round: attempts used (1-3) and whether it was found.
export function record(id, attempts, solved) {
  let r = rec(id);
  const fresh = !r;
  if (!r) r = state.knowledge[id] = { b: 0, due: 0, seen: 0, correct: 0, wrong: 0, lapses: 0, last: 0 };
  const before = fresh ? -1 : r.b;
  if (fresh) r.b = !solved ? 0 : attempts === 1 ? 3 : attempts === 2 ? 1 : 0;
  else if (!solved) {
    r.b = Math.max(0, r.b - 2);
    r.lapses++;
  } else if (attempts === 1) r.b = Math.min(MAX_LEVEL, r.b + 1);
  else if (attempts === 3) r.b = Math.max(0, r.b - 1);
  else if (r.b === 0) r.b = 1;
  r.seen++;
  solved ? r.correct++ : r.wrong++;
  r.last = Date.now();
  r.due = r.last + WAIT[r.b] * (0.85 + Math.random() * 0.3);
  save();
  return { before, after: r.b, due: r.due };
}

// The next country to ask: the most overdue, else a new one (big and easy
// ones first, with a little variety), else the one due soonest.
export function next(pool, lastId) {
  const now = Date.now();
  const due = pool.filter((c) => rec(c.id) && rec(c.id).due <= now).sort((a, b) => rec(a.id).due - rec(b.id).due);
  if (due.length) return due.length > 1 && due[0].id === lastId ? due[1] : due[0];
  const fresh = pool.filter((c) => !rec(c.id));
  if (fresh.length) {
    const order = { easy: 0, medium: 1, hard: 2 };
    fresh.sort((a, b) => order[a.diff] - order[b.diff] || b.area - a.area);
    const front = fresh.slice(0, 4);
    return front[Math.floor(Math.random() * front.length)];
  }
  const soon = pool.filter((c) => c.id !== lastId).sort((a, b) => rec(a.id).due - rec(b.id).due);
  return soon[0] || pool[0];
}

export const dueCount = (pool) => pool.filter((c) => rec(c.id) && rec(c.id).due <= Date.now()).length;

export function summary(pool) {
  const s = { total: pool.length, new: 0, learning: 0, known: 0, mastered: 0, points: 0 };
  for (const c of pool) {
    const l = levelOf(c.id);
    s[category(l)]++;
    if (l > 0) s.points += l;
  }
  s.pct = s.total ? Math.round((s.points / (MAX_LEVEL * s.total)) * 100) : 0;
  return s;
}

export function weakest(pool, n = 5) {
  return pool
    .filter((c) => rec(c.id) && rec(c.id).b <= 2)
    .sort((a, b) => rec(a.id).b - rec(b.id).b || rec(b.id).lapses - rec(a.id).lapses || rec(b.id).wrong - rec(a.id).wrong)
    .slice(0, n);
}

export function untilLabel(ts) {
  const d = ts - Date.now();
  if (d < MIN) return 'in a moment';
  if (d < HOUR) return `in ${Math.round(d / MIN)} min`;
  if (d < DAY) return `in ${Math.round(d / HOUR)} h`;
  const days = Math.round(d / DAY);
  return days === 1 ? 'tomorrow' : `in ${days} days`;
}
