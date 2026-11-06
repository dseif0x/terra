// One game session: a daily, a Blitz, a duel, a practice run or Learn.
// It decides what to ask, judges taps and keeps the score; the component
// shows it and the board (the map) gives feedback.
import { nearest, bearing, arrow, compassWord } from './geo.js';
import { dailyCountries, blitzCountries, shuffle, todayKey } from './rng.js';
import { state, save, finishDaily } from './store.js';
import * as learn from './learn.js';
import { sfx } from './sound.js';
import { fmt } from './data.js';

export const POINTS = [1000, 650, 350]; // by the try that found it
export const MAX_TRIES = 3;
export const ROUNDS = 5;
export const MAX_SCORE = ROUNDS * POINTS[0];
const COAST_KM = 50; // a tap in the sea this close to the target finds it
const SMALL_KM = 16; // any tap this close to a small country finds it
const CLOSE_KM = 80;
export const BLITZ_MS = 20000;
const BLITZ_FLOOR = 200;
const GRACE_MS = 350; // taps right after a round opens are ignored

// points for a country not found in three tries, by the closest tap
const consolation = (km) => Math.round(Math.max(0, 150 * (1 - km / 3000)));

export function emojiOf(r, kind) {
  if (kind === 'blitz') return !r.solved ? '⬛' : r.points >= 600 ? '⚡' : '🟩';
  if (!r.solved) return '🟥';
  return ['🟩', '🟨', '🟧'][r.attempts - 1];
}

export class Session {
  phase = $state('playing'); // playing | reveal | done
  current = $state(null);
  rounds = $state([]);
  score = $state(0);
  found = $state(0);
  toast = $state(null); // { title, text }
  timeLeft = $state(0);
  learnNote = $state(null);

  constructor(data, board, opts) {
    this.data = data;
    this.board = board;
    Object.assign(this, opts); // kind, prompt, region, diff, day, archive, duel
    this.prompt ??= 'classic';
    if (this.kind === 'daily' || this.kind === 'duel') this.targets = dailyCountries(data.countries, this.day);
    if (this.kind === 'blitz') this.targets = blitzCountries(data.countries, this.day);
    if (this.kind === 'practice') this.queue = shuffle(this.pool);
    this.qi = 0;
    this.restore();
  }

  get fixed() {
    return this.targets != null;
  }
  get total() {
    return this.fixed ? this.targets.length : null;
  }

  // a daily or Blitz picks up where it was left, so reloading cannot
  // replay a round
  restore() {
    if (this.kind === 'daily') {
      const saved = state.daily[this.day];
      if (saved && !saved.done) {
        this.rounds = saved.rounds || [];
        this.score = this.rounds.reduce((s, r) => s + r.points, 0);
        this.resume = saved.cur || null;
      }
    }
    if (this.kind === 'blitz') {
      const saved = state.blitz[this.day];
      if (saved && !saved.done) {
        this.rounds = saved.rounds || [];
        // a round that was shown and left unanswered is lost
        if (saved.pending === this.rounds.length && this.rounds.length < ROUNDS) {
          const c = this.targets[this.rounds.length];
          this.rounds.push(this.entry(c, { attempts: 1, solved: false, points: 0, timeout: true }));
        }
        this.score = this.rounds.reduce((s, r) => s + r.points, 0);
      }
    }
    this.found = this.rounds.filter((r) => r.solved).length;
  }

  start() {
    if (this.fixed && this.rounds.length >= this.targets.length) return this.finish();
    this.begin();
  }

  begin() {
    let c;
    if (this.fixed) c = this.targets[this.rounds.length];
    else if (this.kind === 'learn') c = learn.next(this.pool, this.lastId);
    else {
      if (this.qi >= this.queue.length) {
        const prev = this.queue.at(-1);
        this.queue = shuffle(this.pool);
        if (this.queue.length > 1 && this.queue[0] === prev) this.queue.push(this.queue.shift());
        this.qi = 0;
      }
      c = this.queue[this.qi++];
    }
    this.lastId = c.id;
    this.current = { c, attempts: 0, solved: false, points: 0, best: Infinity, wrong: [], pardon: false };
    if (this.resume && this.resume.id === c.id) {
      Object.assign(this.current, { attempts: this.resume.attempts, best: this.resume.best ?? Infinity, wrong: this.resume.wrong || [] });
    }
    this.resume = null;
    this.opened = performance.now();
    this.phase = 'playing';
    this.toast = null;
    this.learnNote = null;
    this.board.clear();
    if (this.kind === 'blitz') {
      state.blitz[this.day] = { done: false, rounds: this.rounds, score: this.score, pending: this.rounds.length };
      save();
      this.deadline = performance.now() + BLITZ_MS;
      this.timeLeft = BLITZ_MS;
      this.lastSec = null;
      clearInterval(this.timer);
      this.timer = setInterval(() => this.tick(), 100);
    }
  }

  tick() {
    if (this.phase !== 'playing') return clearInterval(this.timer);
    const left = Math.max(0, this.deadline - performance.now());
    this.timeLeft = left;
    const sec = Math.ceil(left / 1000);
    if (left > 0 && sec <= 3 && sec !== this.lastSec) {
      this.lastSec = sec;
      sfx.tick();
    }
    if (left <= 0) {
      clearInterval(this.timer);
      Object.assign(this.current, { attempts: 1, solved: false, points: 0, timeout: true });
      sfx.fail();
      this.end();
    }
  }

  // a tap on the globe: the area tapped (or null for the sea) and where
  tap(idx, lngLat) {
    if (this.phase !== 'playing') return;
    if (performance.now() - this.opened < GRACE_MS) return;
    const r = this.current;
    const target = r.c;
    const tapped = this.data.areaAt(idx);
    const ll = [lngLat.lng, lngLat.lat];
    const hit = tapped?.idx === target.idx;
    const near = hit ? { km: 0, point: ll } : nearest(ll, this.data.feature(target.idx));
    const km = Math.round(near.km);
    const coast = !hit && !tapped && near.km <= COAST_KM;
    const small = !hit && !coast && target.small && near.km <= SMALL_KM;
    const correct = hit || coast || small;

    // the first tap into open water in a round is free, and says nothing
    // about where the country is
    if (!correct && !tapped && !r.pardon) {
      r.pardon = true;
      sfx.tap();
      this.toast = {
        title: "That's open water",
        text: this.board.zoom() < 2 ? 'free pass: zoom in for small countries, then tap' : 'free pass, aim for land',
      };
      return;
    }

    if (this.kind === 'blitz') {
      clearInterval(this.timer);
      r.attempts = 1;
      r.solved = correct;
      r.points = correct ? Math.round(BLITZ_FLOOR + (1000 - BLITZ_FLOOR) * (Math.max(0, this.deadline - performance.now()) / BLITZ_MS)) : 0;
      if (!correct) {
        r.best = km;
        if (tapped) this.board.flash(tapped.idx);
      }
      correct ? sfx.right() : sfx.fail();
      return this.end();
    }

    r.attempts++;
    if (correct) {
      r.solved = true;
      r.points = POINTS[r.attempts - 1];
      sfx.right();
      return this.end();
    }
    if (tapped) {
      r.wrong.push(tapped.idx);
      this.board.flash(tapped.idx);
    }
    r.best = Math.min(r.best, km);
    sfx.wrong();
    if (r.attempts >= MAX_TRIES) {
      r.points = consolation(r.best);
      sfx.fail();
      return this.end();
    }
    this.persistCurrent();
    const left = MAX_TRIES - r.attempts;
    const deg = bearing(ll, near.point);
    const who = !tapped
      ? "That's open water"
      : `That's ${tapped.name}` + (tapped.owner ? ` (${tapped.owner})` : '');
    this.toast = {
      title: who,
      flag: tapped && !tapped.owner && tapped.iso2 ? tapped : null,
      text:
        (km < CLOSE_KM ? `so close, within ${CLOSE_KM} km` : `${fmt(km)} km away ${arrow(deg)}`) +
        ` · ${left} ${left === 1 ? 'try' : 'tries'} left`,
      aria: km < CLOSE_KM ? '' : `${compassWord(deg)}`,
    };
  }

  persistCurrent() {
    if (this.kind !== 'daily') return;
    const r = this.current;
    state.daily[this.day] = {
      done: false,
      rounds: this.rounds,
      archive: this.archive,
      cur: { id: r.c.id, attempts: r.attempts, best: Number.isFinite(r.best) ? r.best : null, wrong: r.wrong },
    };
    save();
  }

  // gives up on the current country (practice and Learn "Next")
  skip() {
    if (this.phase !== 'playing') return;
    const r = this.current;
    r.attempts = MAX_TRIES;
    r.solved = false;
    r.points = 0;
    r.skipped = true;
    this.end();
  }

  entry(c, r) {
    return { id: c.id, name: c.name, attempts: r.attempts, solved: r.solved, points: r.points, timeout: !!r.timeout };
  }

  end() {
    const r = this.current;
    clearInterval(this.timer);
    this.toast = null;
    this.score += r.points;
    if (r.solved) this.found++;
    this.rounds = [...this.rounds, this.entry(r.c, r)];
    const graded = learn.record(r.c.id, Math.max(1, r.attempts), r.solved);
    if (this.kind === 'learn') this.learnNote = graded;
    if (this.kind === 'practice') {
      state.practice.rounds++;
      if (r.solved) state.practice.found++;
    }
    if (this.kind === 'daily') {
      state.daily[this.day] = { done: false, rounds: this.rounds, archive: this.archive };
    }
    if (this.kind === 'blitz') {
      state.blitz[this.day] = { done: false, rounds: this.rounds, score: this.score, pending: null };
    }
    save();
    this.board.reveal(r.c, r.solved);
    this.phase = 'reveal';
  }

  next() {
    if (this.phase !== 'reveal') return;
    if (this.fixed && this.rounds.length >= this.targets.length) return this.finish();
    this.begin();
  }

  finish() {
    this.phase = 'done';
    this.board.clear();
    const result = { score: this.score, rounds: this.rounds };
    if (this.kind === 'daily') finishDaily(this.day, { ...result, archive: !!this.archive || this.day !== todayKey() });
    if (this.kind === 'blitz') {
      state.blitz[this.day] = { ...result, done: true };
      save();
    }
    sfx.done();
  }

  stop() {
    clearInterval(this.timer);
    this.board.clear();
  }
}
