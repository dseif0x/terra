// What the game remembers, in this browser: what you know (for Learn),
// daily and Blitz results, streaks and counts.
import { todayKey, addDays } from './rng.js';

const KEY = 'terra.game';

function load() {
  try {
    const s = JSON.parse(localStorage.getItem(KEY));
    if (s && typeof s === 'object') return s;
  } catch {}
  return {};
}

export const state = Object.assign(
  {
    knowledge: {}, // country id -> { b: level, due, seen, correct, wrong, lapses, last }
    daily: {}, // day key -> { done, score, rounds, archive }
    blitz: {}, // day key -> { done, score, rounds }
    streak: { current: 0, best: 0, last: '' },
    practice: { rounds: 0, found: 0 },
    sound: true,
    practiceCfg: { mode: 'classic', region: 'all', diff: 'all' },
    learnRegion: 'all',
  },
  load(),
);

export function save() {
  try {
    localStorage.setItem(KEY, JSON.stringify(state));
  } catch {}
}

// The streak counts days in a row with today's daily finished on the day
// (archive plays do not count). Read it through this, so a missed day
// shows as 0 before the next daily is played.
export function streak() {
  const s = state.streak;
  const t = todayKey();
  if (s.last !== t && s.last !== addDays(t, -1)) return { current: 0, best: s.best };
  return { current: s.current, best: s.best };
}

export function finishDaily(key, result) {
  state.daily[key] = { ...result, done: true };
  if (!result.archive && key === todayKey()) {
    const s = state.streak;
    if (s.last !== key) {
      s.current = s.last === addDays(key, -1) ? s.current + 1 : 1;
      s.last = key;
      s.best = Math.max(s.best, s.current);
    }
  }
  save();
}

export function dailyStats() {
  const done = Object.values(state.daily).filter((d) => d.done);
  const scores = done.map((d) => d.score);
  const buckets = [0, 0, 0, 0, 0, 0]; // 0-999 ... 5000
  for (const s of scores) buckets[Math.min(5, Math.floor(s / 1000))]++;
  return {
    played: done.length,
    perfect: done.filter((d) => d.rounds.every((r) => r.solved && r.attempts === 1)).length,
    average: scores.length ? Math.round(scores.reduce((a, b) => a + b, 0) / scores.length) : 0,
    best: scores.length ? Math.max(...scores) : 0,
    found: done.reduce((n, d) => n + d.rounds.filter((r) => r.solved).length, 0),
    buckets,
  };
}
