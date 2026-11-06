// Small synthesised sounds (no audio files).
import { state } from './store.js';

let ctx;
function tone(freq, start, dur, type = 'sine', gain = 0.12) {
  ctx ??= new (window.AudioContext || window.webkitAudioContext)();
  const t = ctx.currentTime + start;
  const o = ctx.createOscillator();
  const g = ctx.createGain();
  o.type = type;
  o.frequency.setValueAtTime(freq, t);
  g.gain.setValueAtTime(0.0001, t);
  g.gain.exponentialRampToValueAtTime(gain, t + 0.015);
  g.gain.exponentialRampToValueAtTime(0.0001, t + dur);
  o.connect(g).connect(ctx.destination);
  o.start(t);
  o.stop(t + dur + 0.05);
}

const play = (fn) => () => {
  if (!state.sound) return;
  try {
    fn();
  } catch {}
};

export const sfx = {
  tap: play(() => tone(660, 0, 0.06, 'triangle', 0.06)),
  right: play(() => {
    tone(660, 0, 0.12);
    tone(880, 0.09, 0.12);
    tone(1320, 0.18, 0.22);
  }),
  wrong: play(() => tone(200, 0, 0.18, 'sawtooth', 0.05)),
  fail: play(() => {
    tone(330, 0, 0.18, 'triangle');
    tone(247, 0.16, 0.3, 'triangle');
  }),
  tick: play(() => tone(1000, 0, 0.04, 'square', 0.03)),
  done: play(() => [523, 659, 784, 1047].forEach((f, i) => tone(f, i * 0.1, 0.25))),
};
