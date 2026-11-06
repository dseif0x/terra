<script>
  // Game mode: find countries on a blank globe. Daily (five a day, the
  // same for everyone), Blitz (one tap each against the clock, after the
  // daily), duels (a friend's daily as a challenge link), Learn (spaced
  // repetition) and Practice (unlimited, by flag, name or shape).
  import { onMount } from 'svelte';
  import Icon from '../components/Icon.svelte';
  import Silhouette from './Silhouette.svelte';
  import { loadData, REGIONS, NEW_REGION, REGION_VIEW, regionLabel, poolFor, flagUrl, fmt } from './data.js';
  import { gameStyle, COLORS } from './style.js';
  import { Session, MAX_TRIES, MAX_SCORE, BLITZ_MS, emojiOf } from './session.svelte.js';
  import { state as store, save, streak as streakOf, dailyStats } from './store.js';
  import * as learn from './learn.js';
  import { todayKey, dayNumber, dateOfKey, addDays, isValidDay, msUntilTomorrow } from './rng.js';

  let { map, onexit, duelParam = null } = $props();

  let data = $state.raw(null);
  let error = $state('');
  let screen = $state('menu'); // menu | practice | learn | progress | archive | play
  let modal = $state(null); // stats | help
  let session = $state.raw(null);
  let duel = $state(null); // an opened challenge: { d, p, t, n }
  let note = $state(''); // a short message on the menu
  let copied = $state('');
  let tick = $state(0); // re-renders time-dependent labels
  let hoverName = $state('');
  let progressRegion = $state('all');

  let cfg = $state({ ...store.practiceCfg });
  let learnRegion = $state(store.learnRegion);
  let sound = $state(store.sound);
  let playerName = $state(store.name || '');

  const today = todayKey();
  let saved = $state(snapshot());
  function snapshot() {
    return {
      daily: store.daily[todayKey()] || null,
      blitz: store.blitz[todayKey()] || null,
      streak: streakOf(),
    };
  }
  const refresh = () => (saved = snapshot());

  // ---- the board: what the session shows on the map ----
  const coloured = new Set();
  let revealed = null;
  const setColor = (idx, c) => {
    map.setFeatureState({ source: 'countries', id: idx }, { c });
    coloured.add(idx);
  };
  const unset = (idx) => {
    map.setFeatureState({ source: 'countries', id: idx }, { c: null });
    coloured.delete(idx);
  };
  const padding = () => {
    const phone = innerWidth < 721;
    return { top: phone ? 170 : 150, bottom: phone ? 260 : 190, left: 40, right: 40 };
  };
  const board = {
    clear() {
      revealed = null;
      for (const idx of [...coloured]) unset(idx);
      if (map.getLayer('game-mark')) map.setFilter('game-mark', ['==', ['id'], -1]);
      if (map.getZoom() > 3.2 && screen === 'play') map.easeTo({ zoom: 2.4, duration: 700 });
    },
    flash(idx) {
      setColor(idx, COLORS.wrong);
      setTimeout(() => {
        if (idx !== revealed) unset(idx);
      }, 900);
    },
    reveal(c, solved) {
      revealed = c.idx;
      setColor(c.idx, solved ? COLORS.right : COLORS.missed);
      map.setFilter('game-mark', ['==', ['id'], c.idx]);
      map.fitBounds(data.frameOf(c), { padding: padding(), maxZoom: c.small ? 7 : 5.5, duration: 1300 });
    },
    zoom: () => map.getZoom(),
  };

  function heat(region) {
    board.clear();
    for (const c of poolFor(data, region)) {
      const l = learn.levelOf(c.id);
      if (l >= 0) setColor(c.idx, learn.LEVEL_COLORS[l]);
    }
  }

  // ---- map setup ----
  let spinning = true;
  let raf;
  function spin() {
    raf = requestAnimationFrame(spin);
    if (!spinning || screen !== 'menu' || modal || map.isMoving()) return;
    const c = map.getCenter();
    map.jumpTo({ center: [c.lng + 0.03, c.lat] });
  }
  const stopSpin = () => (spinning = false);

  function onClick(e) {
    if (screen !== 'play' || !session) return;
    const f = map.queryRenderedFeatures(e.point, { layers: ['game-fill'] })[0];
    session.tap(f ? f.id : null, e.lngLat);
  }
  let hovered = null;
  function onMove(e) {
    const f = map.queryRenderedFeatures(e.point, { layers: ['game-fill'] })[0];
    const id = f ? f.id : null;
    if (id === hovered) return;
    if (hovered != null) map.setFeatureState({ source: 'countries', id: hovered }, { hover: false });
    hovered = id;
    if (id != null) map.setFeatureState({ source: 'countries', id }, { hover: true });
    map.getCanvas().style.cursor = id != null && screen === 'play' ? 'pointer' : '';
    if (screen === 'progress' && id != null) {
      const a = data.areaAt(id);
      const l = a.iso2 ? learn.levelOf(a.id) : -1;
      hoverName = a.name + (a.iso2 ? ` · ${l < 0 ? 'not seen yet' : learn.category(l) + ', level ' + l}` : '');
    } else hoverName = '';
  }

  onMount(() => {
    let alive = true;
    const prev = {
      dbl: map.doubleClickZoom.isEnabled(),
      rot: map.dragRotate.isEnabled(),
    };
    map.doubleClickZoom.disable();
    map.dragRotate.disable();
    map.touchZoomRotate.disableRotation();
    map.keyboard.disableRotation?.();
    loadData()
      .then((d) => {
        if (!alive) return;
        data = d;
        map.setStyle(gameStyle(d.features), { diff: false });
        map.easeTo({ pitch: 0, bearing: 0, zoom: Math.min(map.getZoom(), 1.6), duration: 600 });
        map.on('click', onClick);
        map.on('mousemove', onMove);
        map.on('mousedown', stopSpin);
        map.on('touchstart', stopSpin);
        map.on('wheel', stopSpin);
        raf = requestAnimationFrame(spin);
        if (duelParam) openDuel(duelParam);
      })
      .catch((e) => (error = e.message));
    const timer = setInterval(() => tick++, 30000);
    const key = (e) => {
      if (e.target.closest?.('input, textarea')) return;
      if (e.key === 'Enter' || e.key === ' ') {
        if (screen === 'play' && session?.phase === 'reveal') {
          e.preventDefault();
          session.next();
        }
      }
      if (e.key === 'Escape') {
        if (modal) modal = null;
        else if (screen === 'play') quit();
        else if (screen !== 'menu') screen = 'menu';
      }
    };
    addEventListener('keydown', key);
    return () => {
      alive = false;
      cancelAnimationFrame(raf);
      clearInterval(timer);
      removeEventListener('keydown', key);
      session?.stop?.();
      map.off('click', onClick);
      map.off('mousemove', onMove);
      map.off('mousedown', stopSpin);
      map.off('touchstart', stopSpin);
      map.off('wheel', stopSpin);
      if (prev.dbl) map.doubleClickZoom.enable();
      if (prev.rot) map.dragRotate.enable();
      map.touchZoomRotate.enableRotation();
      map.keyboard.enableRotation?.();
      map.getCanvas().style.cursor = '';
    };
  });

  // ---- starting things ----
  function view(region) {
    const [lng, lat, zoom] = REGION_VIEW[region] || REGION_VIEW.all;
    const z = innerWidth < 721 ? zoom - 0.25 : zoom;
    map.flyTo({ center: [lng, lat], zoom: z, duration: 1100 });
  }

  function play(opts) {
    session?.stop?.();
    spinning = false;
    session = new Session(data, board, opts);
    screen = 'play';
    session.start();
    if (session.phase !== 'done') view(opts.region || 'all');
  }

  function playDaily(day = today) {
    const archive = day !== today;
    play({ kind: 'daily', day, archive });
  }
  function playBlitz() {
    if (!saved.daily?.done) {
      note = 'Blitz opens once today’s daily is done.';
      return;
    }
    play({ kind: 'blitz', day: today });
  }
  function startPractice() {
    store.practiceCfg = { ...cfg };
    save();
    play({ kind: 'practice', prompt: cfg.mode, region: cfg.region, diff: cfg.diff, pool: poolFor(data, cfg.region, cfg.diff) });
  }
  function startLearn() {
    store.learnRegion = learnRegion;
    save();
    play({ kind: 'learn', region: learnRegion, pool: poolFor(data, learnRegion) });
  }

  function openDuel(param) {
    try {
      const d = JSON.parse(atob(param.replace(/-/g, '+').replace(/_/g, '/')));
      if (!isValidDay(d.d) || typeof d.p !== 'number') throw new Error();
      duel = { d: d.d, p: d.p, t: String(d.t || ''), n: String(d.n || '').slice(0, 24) };
    } catch {
      note = 'That challenge link is broken.';
    }
  }
  function acceptDuel() {
    play({ kind: 'duel', day: duel.d, duel });
    duel = null;
  }

  function quit() {
    session?.stop?.();
    session = null;
    screen = 'menu';
    refresh();
    spinning = true;
    map.easeTo({ zoom: 1.6, duration: 800 });
  }

  function showArchived(day) {
    const d = store.daily[day];
    if (d?.done) {
      session = { phase: 'done', kind: 'daily', day, archive: day !== today, rounds: d.rounds, score: d.score, readOnly: true };
      screen = 'play';
    } else playDaily(day);
  }

  function openProgress() {
    progressRegion = learnRegion;
    screen = 'progress';
    heat(progressRegion);
    view(progressRegion);
  }
  $effect(() => {
    if (screen !== 'progress' && screen !== 'play' && data) {
      // leaving the progress map
      board.clear();
    }
  });

  // ---- sharing ----
  const site = () => location.origin;
  const tiersOf = (rounds) => rounds.map((r) => (r.solved ? String(Math.min(r.attempts, 3)) : '0')).join('');
  function duelLink(day, score, rounds) {
    const payload = { v: 1, d: day, p: score, t: tiersOf(rounds), n: playerName.trim() };
    const b = btoa(JSON.stringify(payload)).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
    return `${site()}/?duel=${b}`;
  }
  function shareText(s) {
    const emoji = s.rounds.map((r) => emojiOf(r, s.kind)).join('');
    if (s.kind === 'blitz') return `terra blitz #${dayNumber(s.day)} · ${fmt(s.score)}\n${emoji}\n${site()}/?game`;
    if (s.kind === 'duel') {
      const them = s.duel.n || 'your challenge';
      return `terra duel #${dayNumber(s.day)} · ${fmt(s.score)} to ${them}'s ${fmt(s.duel.p)}\n${emoji}\n${duelLink(s.day, s.score, s.rounds)}`;
    }
    return `terra daily #${dayNumber(s.day)} · ${fmt(s.score)}/${fmt(MAX_SCORE)}\n${emoji}\n${site()}/?game`;
  }
  async function share(text, what) {
    try {
      if (navigator.share && matchMedia('(pointer: coarse)').matches) await navigator.share({ text });
      else await navigator.clipboard.writeText(text);
      copied = what;
      setTimeout(() => (copied = ''), 2000);
    } catch {}
  }
  function challenge(s) {
    store.name = playerName.trim();
    save();
    const text = `I scored ${fmt(s.score)}/${fmt(MAX_SCORE)} on terra's daily five. Can you beat it?\n${duelLink(s.day, s.score, s.rounds)}`;
    share(text, 'duel');
  }

  // ---- labels ----
  const dayLabel = (key) => dateOfKey(key).toLocaleDateString('en-GB', { weekday: 'short', day: 'numeric', month: 'short' });
  const untilTomorrow = () => {
    tick;
    const m = Math.round(msUntilTomorrow() / 60000);
    return m >= 60 ? `${Math.floor(m / 60)} h ${m % 60} min` : `${m} min`;
  };
  const GRADE = ['Spot on', 'Second try', 'Third try'];
  const gradeOf = (r) =>
    r.timeout ? 'Out of time' : r.skipped ? 'Skipped' : r.solved ? GRADE[r.attempts - 1] : 'Not this time';
  const promptKind = (s) =>
    s.prompt === 'flags' ? 'Find the country with this flag' : s.prompt === 'shapes' ? 'Find the country with this shape' : 'Find';

  const learnPool = $derived(data ? poolFor(data, learnRegion) : []);
  const learnSum = $derived.by(() => {
    tick;
    screen;
    return learnPool.length ? { ...learn.summary(learnPool), due: learn.dueCount(learnPool) } : null;
  });
  const practiceCount = $derived(data ? poolFor(data, cfg.region, cfg.diff).length : 0);
  const progressPool = $derived(data ? poolFor(data, progressRegion) : []);
  const progressSum = $derived.by(() => {
    screen;
    return progressPool.length ? learn.summary(progressPool) : null;
  });
  const worldSum = $derived.by(() => {
    screen;
    tick;
    return data ? { ...learn.summary(data.countries), due: learn.dueCount(data.countries) } : null;
  });

  function factsOf(c) {
    const f = [];
    if (c.capital) f.push(['Capital', c.capital]);
    f.push(['Area', `${fmt(c.area)} km²`]);
    f.push(['Region', c.subregion || c.region]);
    return f;
  }

  const confetti = $derived(
    session?.phase === 'done' && !session.readOnly && session.rounds.length && session.rounds.every((r) => r.solved && r.attempts === 1),
  );
  const archiveDays = $derived.by(() => {
    screen;
    const out = [];
    for (let i = 0; i < 60; i++) {
      const k = addDays(today, -i);
      if (dayNumber(k) < 1) break;
      out.push({ key: k, n: dayNumber(k), saved: store.daily[k] || null });
    }
    return out;
  });
</script>

<!-- top bar -->
<div class="topbar glass">
  <button class="back" onclick={onexit} title="Back to Explore"><Icon name="left" /> Explore</button>
  <div class="context">
    {#if screen === 'play' && session}
      {#if session.kind === 'daily'}Daily #{dayNumber(session.day)}{session.archive ? ' · ' + dayLabel(session.day) : ''}
      {:else if session.kind === 'blitz'}Blitz #{dayNumber(session.day)}
      {:else if session.kind === 'duel'}Duel · daily #{dayNumber(session.day)}
      {:else if session.kind === 'learn'}Learn · {regionLabel(session.region)}
      {:else}Practice · {regionLabel(session.region)}{/if}
    {:else}Game{/if}
  </div>
  <button
    class="icon"
    title={sound ? 'Sound on' : 'Sound off'}
    aria-label="Sound"
    aria-pressed={sound}
    onclick={() => {
      sound = !sound;
      store.sound = sound;
      save();
    }}><Icon name={sound ? 'volume' : 'mute'} size={18} /></button
  >
  <button class="icon" title="Statistics" aria-label="Statistics" onclick={() => (modal = 'stats')}
    ><Icon name="stats" size={18} /></button
  >
  <button class="icon" title="How to play" aria-label="How to play" onclick={() => (modal = 'help')}>?</button>
</div>

{#if error}
  <div class="center glass card">The game could not load: {error}</div>
{:else if !data}
  <div class="center glass card loading"><Icon name="loader" /> Loading the globe…</div>
{:else if duel}
  <div class="center glass card">
    <h2><Icon name="swords" /> A challenge</h2>
    <p>
      {duel.n || 'Someone'} scored <b>{fmt(duel.p)}</b> of {fmt(MAX_SCORE)} on daily #{dayNumber(duel.d)} ({dayLabel(duel.d)}).
      Play the same five and see who wins.
    </p>
    <p class="muted small">A duel does not count towards your daily or streak.</p>
    <div class="row">
      <button class="btn" onclick={() => (duel = null)}>Not now</button>
      <button class="btn primary" onclick={acceptDuel}>Take it on</button>
    </div>
  </div>
{:else if screen === 'menu'}
  <div class="menu">
    <button class="mode glass" onclick={() => playDaily()}>
      <span class="ico daily"><Icon name="calendar" /></span>
      <span class="body">
        <b>Daily #{dayNumber(today)}</b>
        <small>
          {#if saved.daily?.done}Done: {fmt(saved.daily.score)} of {fmt(MAX_SCORE)} · next in {untilTomorrow()}
          {:else if saved.daily?.rounds?.length}Carry on: {saved.daily.rounds.length} of 5 played
          {:else}Five countries, the same for everyone. One go a day.{/if}
        </small>
      </span>
      {#if saved.daily?.done}<span class="emoji">{saved.daily.rounds.map((r) => emojiOf(r)).join('')}</span>{/if}
    </button>
    <button class="mode glass" class:locked={!saved.daily?.done} onclick={playBlitz}>
      <span class="ico blitz"><Icon name="zap" /></span>
      <span class="body">
        <b>Blitz</b>
        <small>
          {#if !saved.daily?.done}Opens after today’s daily: one tap per country, 20 seconds each
          {:else if saved.blitz?.done}Done: {fmt(saved.blitz.score)} points
          {:else if saved.blitz?.rounds?.length}Carry on: {saved.blitz.rounds.length} of 5 played
          {:else}Five more, one tap each, against the clock{/if}
        </small>
      </span>
    </button>
    <button class="mode glass" onclick={() => (screen = 'learn')}>
      <span class="ico learn"><Icon name="learn" /></span>
      <span class="body">
        <b>Learn the world</b>
        <small>
          Spaced repetition: misses come back in minutes, what you know in days
          {#if worldSum?.points}<br />{worldSum.pct}% of the way · {worldSum.due} due now{/if}
        </small>
      </span>
    </button>
    <button class="mode glass" onclick={() => (screen = 'practice')}>
      <span class="ico practice"><Icon name="practice" /></span>
      <span class="body">
        <b>Practice</b>
        <small>Unlimited rounds by name, flag or shape</small>
      </span>
    </button>
    <div class="menu-foot glass">
      <span><Icon name="flame" size={16} /> Streak {saved.streak.current}{saved.streak.best > saved.streak.current ? ` · best ${saved.streak.best}` : ''}</span>
      <button class="link" onclick={() => (screen = 'archive')}>Past dailies</button>
    </div>
    {#if note}<p class="note glass" role="status">{note}</p>{/if}
  </div>
{:else if screen === 'practice'}
  <div class="center glass card setup">
    <h2>Practice</h2>
    <div class="field">
      <span>Ask by</span>
      <div class="seg">
        {#each [['classic', 'Name + flag'], ['flags', 'Flag'], ['shapes', 'Shape']] as [k, l]}
          <button class:on={cfg.mode === k} onclick={() => (cfg.mode = k)}>{l}</button>
        {/each}
      </div>
    </div>
    <div class="field">
      <span>Region</span>
      <div class="seg wrap">
        {#each REGIONS as r}
          <button
            class:on={cfg.region === r.key}
            onclick={() => {
              cfg.region = r.key;
              view(r.key);
            }}>{r.label}</button
          >
        {/each}
      </div>
    </div>
    <div class="field">
      <span>Difficulty</span>
      <div class="seg">
        {#each [['all', 'Mixed'], ['easy', 'Easy'], ['medium', 'Medium'], ['hard', 'Hard']] as [k, l]}
          <button class:on={cfg.diff === k} onclick={() => (cfg.diff = k)}>{l}</button>
        {/each}
      </div>
    </div>
    <p class="muted small">{practiceCount} {practiceCount === 1 ? 'country' : 'countries'}</p>
    <div class="row">
      <button class="btn" onclick={() => (screen = 'menu')}>Back</button>
      <button class="btn primary" disabled={!practiceCount} onclick={startPractice}>Start practising</button>
    </div>
  </div>
{:else if screen === 'learn'}
  <div class="center glass card setup">
    <h2>Learn the world</h2>
    <div class="field">
      <span>Region</span>
      <div class="seg wrap">
        {#each [...REGIONS, NEW_REGION] as r}
          <button
            class:on={learnRegion === r.key}
            onclick={() => {
              learnRegion = r.key;
              view(r.key);
            }}>{r.label}</button
          >
        {/each}
      </div>
    </div>
    {#if learnSum}
      <div class="bar" title="{learnSum.pct}% of the way to mastering {regionLabel(learnRegion)}">
        <span style="width: {learnSum.pct}%"></span>
      </div>
      <p class="small">
        <b>{learnSum.pct}%</b> of the way to mastering {regionLabel(learnRegion)} ·
        {learnSum.mastered} mastered · {learnSum.known} known · {learnSum.learning} learning · {learnSum.new} new
        {#if learnSum.due}<br /><b>{learnSum.due}</b> due for review{/if}
      </p>
    {/if}
    <p class="muted small">What you learn counts everywhere: a country learnt in Europe is known in World too.</p>
    <div class="row">
      <button class="btn" onclick={() => (screen = 'menu')}>Back</button>
      <button class="btn" onclick={openProgress}><Icon name="globe" size={16} /> Progress map</button>
      <button class="btn primary" onclick={startLearn}>Start</button>
    </div>
  </div>
{:else if screen === 'progress'}
  <div class="progress glass card">
    <div class="seg wrap">
      {#each [...REGIONS, NEW_REGION] as r}
        <button
          class:on={progressRegion === r.key}
          onclick={() => {
            progressRegion = r.key;
            heat(r.key);
            view(r.key);
          }}>{r.label}</button
        >
      {/each}
    </div>
    {#if progressSum}
      <p class="small">
        <b>{progressSum.pct}%</b> · {progressSum.mastered} mastered · {progressSum.known} known · {progressSum.learning} learning ·
        {progressSum.new} not seen
      </p>
    {/if}
    <div class="legend" aria-label="Levels 0 to 6">
      <span class="muted small">learning</span>
      {#each learn.LEVEL_COLORS as c, i}<i style="background: {c}" title="Level {i}"></i>{/each}
      <span class="muted small">mastered</span>
    </div>
    {#if learn.weakest(progressPool).length}
      <p class="small weak">
        Needs work: {learn
          .weakest(progressPool)
          .map((c) => c.name)
          .join(', ')}
      </p>
    {/if}
    {#if hoverName}<p class="small hover">{hoverName}</p>{/if}
    <div class="row">
      <button class="btn" onclick={() => (screen = 'learn')}>Back</button>
    </div>
  </div>
{:else if screen === 'archive'}
  <div class="center glass card archive">
    <h2>Past dailies</h2>
    <p class="muted small">Play any day you missed. Past days don’t count towards the streak.</p>
    <ul>
      {#each archiveDays as d}
        <li>
          <button onclick={() => (d.key === today ? playDaily() : showArchived(d.key))}>
            <span class="n">#{d.n}</span>
            <span class="date">{d.key === today ? 'Today' : dayLabel(d.key)}</span>
            <span class="res">
              {#if d.saved?.done}{d.saved.rounds.map((r) => emojiOf(r)).join('')} {fmt(d.saved.score)}
              {:else if d.saved?.rounds?.length}{d.saved.rounds.length}/5 played
              {:else}<span class="muted">Play</span>{/if}
            </span>
          </button>
        </li>
      {/each}
    </ul>
    <div class="row"><button class="btn" onclick={() => (screen = 'menu')}>Back</button></div>
  </div>
{:else if screen === 'play' && session}
  {#if session.phase === 'done'}
    {@const s = session}
    <div class="center glass card results">
      {#if s.kind === 'duel'}
        {@const won = s.score > s.duel.p}
        {@const tie = s.score === s.duel.p}
        <h2><Icon name="swords" /> {won ? 'You win' : tie ? 'A dead heat' : `${s.duel.n || 'They'} won`}</h2>
        <div class="versus">
          <div><small>You</small><b>{fmt(s.score)}</b></div>
          <div><small>{s.duel.n || 'Them'}</small><b>{fmt(s.duel.p)}</b></div>
        </div>
        <p class="emoji big">{s.rounds.map((r) => emojiOf(r)).join('')}</p>
        {#if s.duel.t}<p class="emoji">{[...s.duel.t].map((t) => (t === '0' ? '🟥' : ['🟩', '🟨', '🟧'][+t - 1])).join('')}</p>{/if}
      {:else}
        <h2>
          {s.kind === 'blitz' ? 'Blitz' : 'Daily'} #{dayNumber(s.day)}
          {#if s.archive}<small class="muted">{dayLabel(s.day)}</small>{/if}
        </h2>
        <p class="score"><b>{fmt(s.score)}</b>{#if s.kind !== 'blitz'}<span class="muted"> / {fmt(MAX_SCORE)}</span>{/if}</p>
        <p class="emoji big">{s.rounds.map((r) => emojiOf(r, s.kind)).join('')}</p>
      {/if}
      <ol class="rounds">
        {#each s.rounds as r}
          {@const c = data.byId.get(r.id)}
          <li>
            <img src={flagUrl(c)} alt="" />
            <span>{c.name}</span>
            <span class="muted">{gradeOf(r)}</span>
            <b>{fmt(r.points)}</b>
          </li>
        {/each}
      </ol>
      {#if s.kind === 'daily' && !s.archive}
        <p class="small"><Icon name="flame" size={15} /> Streak {streakOf().current} · next daily in {untilTomorrow()}</p>
      {/if}
      {#if s.kind === 'daily'}
        <label class="namefield">
          <span class="small muted">Your name on challenge links</span>
          <input bind:value={playerName} maxlength="24" placeholder="optional" />
        </label>
      {/if}
      <div class="row wrap">
        <button class="btn" onclick={quit}>Menu</button>
        <button class="btn" onclick={() => share(shareText(s), 'share')}
          ><Icon name={copied === 'share' ? 'check' : 'share'} size={16} /> {copied === 'share' ? 'Copied' : 'Share'}</button
        >
        {#if s.kind === 'daily'}
          <button class="btn" onclick={() => challenge(s)}
            ><Icon name={copied === 'duel' ? 'check' : 'swords'} size={16} /> {copied === 'duel' ? 'Link copied' : 'Challenge a friend'}</button
          >
        {/if}
        {#if s.kind === 'duel'}
          <button class="btn" onclick={() => challenge(s)}
            ><Icon name={copied === 'duel' ? 'check' : 'swords'} size={16} /> {copied === 'duel' ? 'Link copied' : 'Challenge back'}</button
          >
        {/if}
        {#if s.kind === 'daily' && !s.archive && !store.blitz[today]?.done}
          <button
            class="btn primary"
            onclick={() => {
              refresh();
              playBlitz();
            }}><Icon name="zap" size={16} /> Blitz</button
          >
        {/if}
      </div>
    </div>
    {#if confetti}
      <div class="confetti" aria-hidden="true">
        {#each Array(60) as _, i}
          <i style="left: {(i * 37) % 100}%; animation-delay: {(i % 12) * 0.08}s; background: hsl({(i * 47) % 360} 80% 60%)"></i>
        {/each}
      </div>
    {/if}
  {:else if session.current}
    {@const s = session}
    {@const r = s.current}
    {@const c = r.c}
    <div class="hud glass">
      <div class="prompt">
        <small class="kind">{promptKind(s)}</small>
        {#if s.prompt === 'flags'}
          <img class="bigflag" src={flagUrl(c)} alt="A flag" />
        {:else if s.prompt === 'shapes'}
          <Silhouette feature={data.feature(c.idx)} color="#e8edf4" />
        {:else}
          <div class="name"><img src={flagUrl(c)} alt="" /><b>{c.name}</b></div>
        {/if}
      </div>
      <div class="meta">
        {#if s.kind !== 'blitz'}
          <div class="pips" aria-label="{MAX_TRIES - r.attempts} tries left">
            {#each Array(MAX_TRIES) as _, i}<i class:used={i < r.attempts}></i>{/each}
          </div>
        {/if}
        {#if s.fixed}
          <span class="small">{s.phase === 'reveal' ? s.rounds.length : s.rounds.length + 1} / {s.total}</span>
          <span class="small"><b>{fmt(s.score)}</b> pts</span>
        {:else if s.kind === 'learn'}
          <span class="small">Found {s.found} / {s.rounds.length}</span>
          {#if learnSum}<span class="small muted">{learnSum.due} due</span>{/if}
        {:else}
          <span class="small">Found <b>{s.found}</b> / {s.rounds.length}</span>
          <span class="small muted" title="Found on the first try in a row">Streak {s.streak} · best {store.practice.best || 0}</span>
        {/if}
      </div>
      {#if s.kind === 'blitz'}
        <div class="timer"><span style="width: {(s.timeLeft / BLITZ_MS) * 100}%"></span></div>
      {/if}
      <div class="hud-actions">
        <button class="btn small" onclick={quit}>Exit</button>
        {#if !s.fixed && s.phase === 'playing'}
          <button class="btn small" onclick={() => s.skip()}><Icon name="skip" size={15} /> Next</button>
        {/if}
      </div>
    </div>

    {#if s.toast && s.phase === 'playing'}
      <div class="toast glass" role="status" aria-label="{s.toast.title}, {s.toast.text} {s.toast.aria || ''}">
        <b>{#if s.toast.flag}<img src={flagUrl(s.toast.flag)} alt="" />{/if}{s.toast.title}</b>
        <small>{s.toast.text}</small>
      </div>
    {/if}

    {#if s.phase === 'reveal'}
      {@const last = s.rounds.at(-1)}
      <div class="reveal glass" class:good={last.solved}>
        <div class="grade">{gradeOf(last)}{#if last.points}&nbsp;<b>+{fmt(last.points)}</b>{/if}</div>
        <div class="who">
          <img src={flagUrl(c)} alt="Flag of {c.name}" />
          <div>
            <b>{c.name}</b>
            {#if c.official !== c.name}<small class="muted">{c.official}</small>{/if}
          </div>
        </div>
        <dl>
          {#each factsOf(c) as [k, v]}<dt>{k}</dt><dd>{v}</dd>{/each}
        </dl>
        {#if s.learnNote}
          <p class="small learn-note">
            {s.learnNote.before < 0 ? 'New: level' : s.learnNote.after > s.learnNote.before ? 'Up to level' : s.learnNote.after < s.learnNote.before ? 'Down to level' : 'Stays at level'}
            {s.learnNote.after} · back {learn.untilLabel(s.learnNote.due)}
          </p>
        {/if}
        <button class="btn primary next" onclick={() => s.next()}>
          {s.fixed && s.rounds.length >= s.total ? 'Results' : 'Next'}<Icon name="right" size={16} />
        </button>
      </div>
    {/if}
  {/if}
{/if}

{#if modal === 'stats' && data}
  {@const st = dailyStats()}
  {@const sk = streakOf()}
  {@const ws = learn.summary(data.countries)}
  {@const bl = Object.values(store.blitz).filter((b) => b.done)}
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <div class="modal" role="dialog" tabindex="-1" aria-label="Statistics" onclick={(e) => e.target === e.currentTarget && (modal = null)}>
    <div class="glass card">
      <h2>Statistics</h2>
      <div class="tiles">
        <div><b>{st.played}</b><small>dailies</small></div>
        <div><b>{fmt(st.average)}</b><small>average</small></div>
        <div><b>{fmt(st.best)}</b><small>best</small></div>
        <div><b>{st.perfect}</b><small>perfect</small></div>
        <div><b>{sk.current}</b><small>streak</small></div>
        <div><b>{sk.best}</b><small>best streak</small></div>
      </div>
      {#if st.played}
        {@const max = Math.max(...st.buckets)}
        <div class="hist" aria-label="Daily scores">
          {#each st.buckets as n, i}
            <div><span class="small muted">{i === 5 ? '5k' : `${i}k+`}</span><i style="width: {max ? (n / max) * 100 : 0}%"></i><span class="small">{n}</span></div>
          {/each}
        </div>
      {/if}
      <p class="small">
        Blitz: {bl.length} played{bl.length ? `, best ${fmt(Math.max(...bl.map((b) => b.score)))}` : ''}<br />
        Practice: {fmt(store.practice.found)} found in {fmt(store.practice.rounds)} rounds · best first-try streak {store.practice.best || 0}<br />
        Learn: {ws.pct}% of the world · {ws.mastered} mastered, {ws.known} known, {ws.learning} learning
      </p>
      <p class="muted small">Kept in this browser only.</p>
      <div class="row"><button class="btn" onclick={() => (modal = null)}>Close</button></div>
    </div>
  </div>
{/if}

{#if modal === 'help'}
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <div class="modal" role="dialog" tabindex="-1" aria-label="How to play" onclick={(e) => e.target === e.currentTarget && (modal = null)}>
    <div class="glass card help">
      <h2>How to play</h2>
      <p>Spin the globe, zoom in and tap the country you are asked for.</p>
      <ul>
        <li>Three tries per country: <b>1,000</b>, <b>650</b> or <b>350</b> points. Miss all three and the closest tap still earns up to 150.</li>
        <li>A wrong tap tells you which country you hit and how far away and in which direction the target is.</li>
        <li>Your first tap into open water each round is free. A tap in the sea within 50 km of the coast counts as found, and for tiny countries anything within 16 km does.</li>
        <li><b>Daily</b>: five countries, from easy to hard, the same for everyone. Keep a streak by playing every day.</li>
        <li><b>Blitz</b>: after the daily, five more with one tap each and 20 seconds; the faster, the more points.</li>
        <li><b>Challenge a friend</b>: send a link to your daily; they play the same five and see who wins.</li>
        <li><b>Learn</b>: spaced repetition. Countries you miss come back within minutes, the ones you know after days.</li>
        <li><b>Practice</b>: endless rounds by name, flag or shape, by region and difficulty.</li>
      </ul>
      <div class="row"><button class="btn" onclick={() => (modal = null)}>Got it</button></div>
    </div>
  </div>
{/if}

<style>
  .topbar {
    position: absolute;
    z-index: 12;
    top: calc(12px + env(safe-area-inset-top));
    left: 12px;
    right: 12px;
    max-width: 560px;
    margin: 0 auto;
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 4px;
    border-radius: 14px;
  }
  .topbar .back {
    gap: 2px;
    padding: 0 10px 0 4px;
    height: 36px;
    border-radius: 10px;
    color: var(--muted);
  }
  .topbar .back:hover,
  .topbar .icon:hover {
    background: var(--hover);
    color: var(--text);
  }
  .context {
    flex: 1;
    text-align: center;
    font-weight: 600;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .topbar .icon {
    width: 36px;
    height: 36px;
    border-radius: 10px;
    color: var(--muted);
    font-weight: 700;
  }
  .card {
    border-radius: 18px;
    padding: 18px 20px;
  }
  .center {
    position: absolute;
    z-index: 11;
    left: 50%;
    top: 50%;
    transform: translate(-50%, -50%);
    width: min(440px, calc(100vw - 32px));
    max-height: calc(100dvh - 120px);
    overflow: auto;
  }
  .loading {
    display: flex;
    gap: 10px;
    align-items: center;
    justify-content: center;
  }
  h2 {
    margin: 0 0 10px;
    font-size: 20px;
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .muted {
    color: var(--muted);
  }
  .small {
    font-size: 13px;
  }
  .row {
    display: flex;
    gap: 8px;
    justify-content: flex-end;
    margin-top: 14px;
  }
  .row.wrap {
    flex-wrap: wrap;
  }
  .btn {
    gap: 6px;
    height: 38px;
    padding: 0 14px;
    border-radius: 10px;
    background: var(--hover);
    font-weight: 600;
  }
  .btn:hover {
    background: var(--hover-strong);
  }
  .btn.primary {
    background: var(--accent);
    color: #04121f;
  }
  .btn.primary:hover {
    filter: brightness(1.08);
  }
  .btn:disabled {
    opacity: 0.4;
    cursor: default;
  }
  .btn.small {
    height: 30px;
    padding: 0 10px;
    font-size: 13px;
  }

  /* menu */
  .menu {
    position: absolute;
    z-index: 11;
    left: 50%;
    bottom: calc(20px + env(safe-area-inset-bottom));
    transform: translateX(-50%);
    width: min(440px, calc(100vw - 32px));
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .mode {
    justify-content: flex-start;
    gap: 12px;
    padding: 12px 14px;
    border-radius: 16px;
    text-align: left;
  }
  .mode:hover {
    background: rgba(34, 40, 50, 0.9);
  }
  .mode.locked {
    opacity: 0.6;
  }
  .mode .body {
    flex: 1;
    display: flex;
    flex-direction: column;
  }
  .mode small {
    color: var(--muted);
    font-size: 13px;
  }
  .ico {
    flex: none;
    width: 40px;
    height: 40px;
    border-radius: 12px;
    display: grid;
    place-items: center;
  }
  .ico.daily {
    background: rgba(94, 176, 255, 0.18);
    color: #5eb0ff;
  }
  .ico.blitz {
    background: rgba(245, 197, 24, 0.18);
    color: #f5c518;
  }
  .ico.learn {
    background: rgba(48, 164, 108, 0.2);
    color: #4cc38a;
  }
  .ico.practice {
    background: rgba(190, 120, 255, 0.18);
    color: #c79bff;
  }
  .emoji {
    letter-spacing: 2px;
    margin: 0;
  }
  .emoji.big {
    font-size: 26px;
    text-align: center;
    margin: 6px 0;
  }
  .menu-foot {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 8px 14px;
    border-radius: 12px;
    font-size: 14px;
  }
  .menu-foot span {
    display: inline-flex;
    gap: 6px;
    align-items: center;
    color: #ffb054;
  }
  .link {
    color: var(--accent);
  }
  .note {
    margin: 0;
    padding: 8px 14px;
    border-radius: 12px;
    font-size: 14px;
  }

  /* setup */
  .field {
    margin: 12px 0;
  }
  .field > span {
    display: block;
    font-size: 13px;
    color: var(--muted);
    margin-bottom: 6px;
  }
  .seg {
    display: flex;
    gap: 4px;
    background: var(--card);
    border: 1px solid var(--line);
    padding: 3px;
    border-radius: 11px;
  }
  .seg.wrap {
    flex-wrap: wrap;
  }
  .seg button {
    flex: 1 0 auto;
    height: 32px;
    padding: 0 10px;
    border-radius: 8px;
    font-size: 14px;
    color: var(--muted);
  }
  .seg button.on {
    background: var(--hover-strong);
    color: var(--text);
    font-weight: 600;
  }
  .bar {
    height: 8px;
    border-radius: 4px;
    background: var(--hover);
    overflow: hidden;
    margin-top: 6px;
  }
  .bar span {
    display: block;
    height: 100%;
    background: linear-gradient(90deg, #e9b949, #5bc77d, #2fb8a4);
  }

  /* progress */
  .progress {
    position: absolute;
    z-index: 11;
    left: 50%;
    bottom: calc(16px + env(safe-area-inset-bottom));
    transform: translateX(-50%);
    width: min(560px, calc(100vw - 32px));
  }
  .progress p {
    margin: 10px 0 0;
  }
  .legend {
    display: flex;
    align-items: center;
    gap: 3px;
    margin-top: 8px;
  }
  .legend i {
    width: 22px;
    height: 10px;
    border-radius: 3px;
  }
  .legend span {
    margin: 0 4px;
  }
  .hover {
    color: var(--accent);
  }

  /* archive */
  .archive ul {
    list-style: none;
    padding: 0;
    margin: 8px 0 0;
    max-height: 50dvh;
    overflow: auto;
  }
  .archive li button {
    width: 100%;
    justify-content: flex-start;
    gap: 10px;
    padding: 8px 6px;
    border-radius: 8px;
    border-bottom: 1px solid var(--line);
  }
  .archive li button:hover {
    background: var(--hover);
  }
  .archive .n {
    width: 48px;
    color: var(--muted);
    text-align: left;
  }
  .archive .date {
    flex: 1;
    text-align: left;
  }

  /* play */
  .hud {
    position: absolute;
    z-index: 11;
    top: calc(64px + env(safe-area-inset-top));
    left: 50%;
    transform: translateX(-50%);
    width: min(440px, calc(100vw - 24px));
    border-radius: 16px;
    padding: 10px 14px;
    display: grid;
    grid-template-columns: 1fr auto;
    gap: 6px 10px;
    align-items: center;
  }
  .prompt {
    display: flex;
    flex-direction: column;
    gap: 4px;
    min-width: 0;
  }
  .kind {
    color: var(--muted);
    font-size: 12px;
    text-transform: uppercase;
    letter-spacing: 0.06em;
  }
  .prompt .name {
    display: flex;
    align-items: center;
    gap: 10px;
    font-size: 22px;
  }
  .prompt .name b {
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .prompt .name img {
    height: 22px;
    border-radius: 3px;
    box-shadow: 0 0 0 1px rgba(255, 255, 255, 0.15);
  }
  .bigflag {
    height: 84px;
    width: auto;
    max-width: 100%;
    align-self: flex-start;
    border-radius: 6px;
    box-shadow: 0 0 0 1px rgba(255, 255, 255, 0.15);
  }
  .prompt :global(svg) {
    width: 180px;
    height: 120px;
  }
  .meta {
    display: flex;
    flex-direction: column;
    align-items: flex-end;
    gap: 4px;
  }
  .pips {
    display: flex;
    gap: 4px;
  }
  .pips i {
    width: 10px;
    height: 10px;
    border-radius: 50%;
    background: #4cc38a;
  }
  .pips i.used {
    background: rgba(255, 255, 255, 0.18);
  }
  .timer {
    grid-column: 1 / -1;
    height: 5px;
    border-radius: 3px;
    background: var(--hover);
    overflow: hidden;
  }
  .timer span {
    display: block;
    height: 100%;
    background: #f5c518;
    transition: width 0.1s linear;
  }
  .hud-actions {
    grid-column: 1 / -1;
    display: flex;
    gap: 6px;
    justify-content: flex-end;
  }
  .toast {
    position: absolute;
    z-index: 11;
    left: 50%;
    bottom: calc(28px + env(safe-area-inset-bottom));
    transform: translateX(-50%);
    padding: 10px 16px;
    border-radius: 14px;
    display: flex;
    flex-direction: column;
    align-items: center;
    max-width: calc(100vw - 32px);
    text-align: center;
    pointer-events: none;
  }
  .toast b {
    display: flex;
    gap: 8px;
    align-items: center;
  }
  .toast img {
    height: 15px;
    border-radius: 2px;
  }
  .toast small {
    color: var(--muted);
  }
  .reveal {
    position: absolute;
    z-index: 11;
    left: 50%;
    bottom: calc(16px + env(safe-area-inset-bottom));
    transform: translateX(-50%);
    width: min(440px, calc(100vw - 24px));
    border-radius: 18px;
    padding: 14px 16px;
    border-top: 3px solid #f5c518;
  }
  .reveal.good {
    border-top-color: #30a46c;
  }
  .grade {
    font-size: 13px;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--muted);
  }
  .grade b {
    color: var(--text);
  }
  .who {
    display: flex;
    gap: 12px;
    align-items: center;
    margin: 8px 0;
  }
  .who img {
    height: 40px;
    border-radius: 4px;
    box-shadow: 0 0 0 1px rgba(255, 255, 255, 0.15);
  }
  .who b {
    display: block;
    font-size: 20px;
  }
  .who small {
    font-size: 13px;
  }
  dl {
    display: grid;
    grid-template-columns: auto 1fr;
    gap: 2px 12px;
    margin: 0;
    font-size: 14px;
  }
  dt {
    color: var(--muted);
  }
  dd {
    margin: 0;
  }
  .learn-note {
    margin: 8px 0 0;
    color: #4cc38a;
  }
  .next {
    width: 100%;
    margin-top: 12px;
  }

  /* results */
  .score {
    text-align: center;
    font-size: 34px;
    margin: 4px 0;
  }
  .score span {
    font-size: 20px;
  }
  .versus {
    display: flex;
    justify-content: space-around;
    text-align: center;
    margin: 8px 0;
  }
  .versus small {
    display: block;
    color: var(--muted);
  }
  .versus b {
    font-size: 28px;
  }
  .rounds {
    list-style: none;
    padding: 0;
    margin: 10px 0;
  }
  .rounds li {
    display: grid;
    grid-template-columns: 26px 1fr auto 56px;
    gap: 10px;
    align-items: center;
    padding: 5px 0;
    border-bottom: 1px solid var(--line);
    font-size: 14px;
  }
  .rounds img {
    width: 26px;
    border-radius: 2px;
  }
  .rounds b {
    text-align: right;
  }
  .namefield {
    display: flex;
    flex-direction: column;
    gap: 4px;
    margin-top: 8px;
  }
  input {
    font: inherit;
    color: var(--text);
    background: var(--card);
    border: 1px solid var(--line);
    border-radius: 9px;
    padding: 7px 10px;
  }

  /* stats / help */
  .modal {
    position: absolute;
    inset: 0;
    z-index: 30;
    background: rgba(0, 0, 0, 0.45);
    display: grid;
    place-items: center;
    padding: 16px;
  }
  .modal .card {
    width: min(440px, 100%);
    max-height: calc(100dvh - 32px);
    overflow: auto;
  }
  .tiles {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 8px;
  }
  .tiles div {
    background: var(--card);
    border: 1px solid var(--line);
    border-radius: 12px;
    padding: 8px;
    text-align: center;
  }
  .tiles b {
    display: block;
    font-size: 20px;
  }
  .tiles small {
    color: var(--muted);
  }
  .hist {
    margin: 12px 0;
    display: flex;
    flex-direction: column;
    gap: 3px;
  }
  .hist div {
    display: grid;
    grid-template-columns: 30px 1fr 24px;
    gap: 6px;
    align-items: center;
  }
  .hist i {
    display: block;
    height: 12px;
    min-width: 2px;
    border-radius: 3px;
    background: var(--accent);
  }
  .help ul {
    padding-left: 18px;
    font-size: 14px;
  }
  .help li {
    margin: 5px 0;
  }

  .confetti {
    position: absolute;
    inset: 0;
    z-index: 25;
    pointer-events: none;
    overflow: hidden;
  }
  .confetti i {
    position: absolute;
    top: -12px;
    width: 8px;
    height: 12px;
    border-radius: 2px;
    animation: fall 2.6s ease-in forwards;
  }
  @keyframes fall {
    to {
      transform: translateY(105vh) rotate(720deg);
      opacity: 0.6;
    }
  }

  @media (max-width: 720px) {
    .hud {
      top: calc(60px + env(safe-area-inset-top));
    }
    .prompt .name {
      font-size: 19px;
    }
    .bigflag {
      height: 64px;
    }
    .prompt :global(svg) {
      width: 140px;
      height: 96px;
    }
  }
</style>
