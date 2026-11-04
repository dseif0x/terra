<script>
  import Icon from './Icon.svelte';
  import { search, parseCoords } from '../lib/api.js';

  // onpick({name, kind, context, lat, lon, osm?, extent?}); bias() gives the
  // map's center and zoom so nearby results come first.
  let { onpick, bias } = $props();

  let q = $state('');
  let results = $state([]);
  let open = $state(false);
  let active = $state(0);
  let busy = $state(false);
  let error = $state('');
  let input;
  let timer;
  let ctrl;

  function run() {
    clearTimeout(timer);
    ctrl?.abort();
    error = '';
    const text = q.trim();
    const c = parseCoords(text);
    if (c) {
      results = [{ name: `${c.lat}, ${c.lon}`, kind: 'Coordinates', lat: c.lat, lon: c.lon, coords: true }];
      active = 0;
      return;
    }
    if (text.length < 2) {
      results = [];
      return;
    }
    timer = setTimeout(async () => {
      ctrl = new AbortController();
      busy = true;
      try {
        const { center, zoom } = bias();
        results = await search(text, center, zoom, ctrl.signal);
        active = 0;
      } catch (e) {
        if (e.name !== 'AbortError') error = e.message;
      } finally {
        busy = false;
      }
    }, 180);
  }

  function pick(r) {
    if (!r) return;
    q = r.coords ? r.name : r.name;
    open = false;
    input?.blur();
    onpick(r);
  }

  function key(e) {
    if (e.key === 'ArrowDown') {
      active = Math.min(active + 1, results.length - 1);
      e.preventDefault();
    } else if (e.key === 'ArrowUp') {
      active = Math.max(active - 1, 0);
      e.preventDefault();
    } else if (e.key === 'Enter') {
      pick(results[active]);
    } else if (e.key === 'Escape') {
      open = false;
      input?.blur();
    }
  }

  function clear() {
    q = '';
    results = [];
    error = '';
    input?.focus();
  }

  export function setText(t) {
    q = t;
  }
</script>

<div class="search glass" class:open={open && (results.length || error)}>
  <div class="row">
    <span class="lead">
      {#if busy}<span class="spin"><Icon name="loader" size={18} /></span>{:else}<Icon name="search" size={18} />{/if}
    </span>
    <input
      bind:this={input}
      bind:value={q}
      oninput={() => {
        open = true;
        run();
      }}
      onfocus={() => (open = true)}
      onblur={() => setTimeout(() => (open = false), 150)}
      onkeydown={key}
      placeholder="Search places, addresses, coordinates"
      autocomplete="off"
      spellcheck="false"
      enterkeyhint="search"
      aria-label="Search"
    />
    {#if q}
      <button class="clear" onclick={clear} aria-label="Clear"><Icon name="x" size={18} /></button>
    {/if}
  </div>
  {#if open && (results.length || error)}
    <ul class="results" role="listbox">
      {#if error}<li class="err">{error}</li>{/if}
      {#each results as r, i (r.osm ?? r.name + i)}
        <li role="option" aria-selected={i === active}>
          <button class:active={i === active} onmousedown={(e) => e.preventDefault()} onclick={() => pick(r)}>
            <span class="name">{r.name}</span>
            <span class="sub">{r.kind}{r.context ? ' · ' + r.context : ''}</span>
          </button>
        </li>
      {/each}
    </ul>
  {/if}
</div>

<style>
  .search {
    border-radius: 14px;
    overflow: hidden;
  }
  .row {
    display: flex;
    align-items: center;
    height: 48px;
    padding: 0 6px 0 14px;
    gap: 10px;
  }
  .lead {
    display: flex;
    color: var(--muted);
  }
  .spin {
    display: flex;
    animation: spin 0.9s linear infinite;
  }
  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }
  input {
    flex: 1;
    min-width: 0;
    background: none;
    border: 0;
    outline: 0;
    color: var(--text);
    font: inherit;
    font-size: 16px;
  }
  input::placeholder {
    color: var(--muted);
  }
  .clear {
    width: 36px;
    height: 36px;
    border-radius: 10px;
    color: var(--muted);
  }
  .results {
    list-style: none;
    margin: 0;
    padding: 4px 0 6px;
    border-top: 1px solid var(--line);
    max-height: min(60vh, 420px);
    overflow-y: auto;
  }
  .results button {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    width: 100%;
    padding: 9px 16px;
    text-align: left;
    gap: 2px;
  }
  .results button.active,
  .results button:hover {
    background: var(--hover);
  }
  .name {
    font-weight: 600;
  }
  .sub {
    font-size: 13px;
    color: var(--muted);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    max-width: 100%;
  }
  .err {
    padding: 10px 16px;
    color: var(--danger);
    font-size: 14px;
  }
</style>
