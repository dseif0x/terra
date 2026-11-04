<script>
  import Icon from './Icon.svelte';

  // place: the backend's Place, or null while loading; preview: {name,
  // kind} known from the click or search before the details arrive.
  let { place, preview, loading, error, onclose, onphoto, onfocus } = $props();

  let expanded = $state(false);
  let copied = $state(false);
  let body;

  const name = $derived(place?.name || preview?.name || 'Dropped pin');
  const sub = $derived(place?.description || place?.kind || preview?.kind || '');
  const coords = $derived(
    place ? `${place.lat.toFixed(5)}, ${place.lon.toFixed(5)}` : preview?.lat != null ? `${preview.lat.toFixed(5)}, ${preview.lon.toFixed(5)}` : '',
  );

  $effect(() => {
    // a new place starts at the top
    place?.osm;
    preview?.name;
    if (body) body.scrollTop = 0;
  });

  async function copy() {
    try {
      await navigator.clipboard.writeText(coords);
      copied = true;
      setTimeout(() => (copied = false), 1200);
    } catch {}
  }

  async function share() {
    const url = location.href;
    if (navigator.share) {
      try {
        await navigator.share({ title: name, url });
      } catch {}
    } else {
      await navigator.clipboard.writeText(url).catch(() => {});
    }
  }

  const isURL = (v) => /^https?:\/\//.test(v);
  const isPhone = (label) => label === 'Phone';
  const host = (u) => {
    try {
      return new URL(u).host.replace(/^www\./, '');
    } catch {
      return u;
    }
  };
</script>

<aside class="panel glass" class:expanded>
  <button class="grip" onclick={() => (expanded = !expanded)} aria-label={expanded ? 'Collapse' : 'Expand'}>
    <span></span>
  </button>
  <header>
    <div class="titles">
      <h1>{name}</h1>
      {#if sub}<p class="sub">{sub}</p>{/if}
      {#if place?.context}<p class="ctx">{place.context}</p>{/if}
    </div>
    <button class="icon-btn" onclick={onclose} aria-label="Close"><Icon name="x" /></button>
  </header>

  <div class="body" bind:this={body}>
    {#if place?.photos?.length}
      <div class="photos">
        {#each place.photos as ph, i (ph.page)}
          <button class="photo" onclick={() => onphoto(i)} title={ph.title}>
            <img src={ph.thumb} alt={ph.title || name} loading={i < 3 ? 'eager' : 'lazy'} referrerpolicy="no-referrer" />
          </button>
        {/each}
      </div>
    {:else if loading}
      <div class="photos"><div class="photo skeleton"></div><div class="photo skeleton"></div><div class="photo skeleton"></div></div>
    {/if}

    <div class="actions">
      {#if place}
        <button class="chip" onclick={onfocus}><Icon name="pin" size={16} /> Show</button>
      {/if}
      <button class="chip" onclick={share}><Icon name="share" size={16} /> Share</button>
      {#if coords}
        <button class="chip mono" onclick={copy} title="Copy coordinates">
          <Icon name={copied ? 'check' : 'copy'} size={16} />
          {coords}
        </button>
      {/if}
    </div>

    {#if error}
      <p class="error">{error}</p>
    {/if}

    {#if loading && !place}
      <div class="lines"><span></span><span></span><span class="short"></span></div>
    {/if}

    {#if place?.facts?.length}
      <dl class="facts">
        {#each place.facts as f (f.label)}
          <div class="fact" class:wide={isURL(f.value)}>
            <dt>{f.label}{#if f.note}<span class="note">&nbsp;({f.note})</span>{/if}</dt>
            <dd>
              {#if isURL(f.value)}<a href={f.value} target="_blank" rel="noopener">{host(f.value)}</a>{:else}{f.value}{/if}
            </dd>
          </div>
        {/each}
      </dl>
    {/if}

    {#if place?.summary}
      <section class="summary">
        <p>{place.summary}</p>
        {#each place.links.filter((l) => l.label === 'Wikipedia') as l}
          <a class="more" href={l.url} target="_blank" rel="noopener">Read more on Wikipedia</a>
        {/each}
      </section>
    {/if}

    {#if place?.address || place?.details?.length}
      <dl class="details">
        {#if place.address}
          <div><dt>Address</dt><dd>{place.address}</dd></div>
        {/if}
        {#each place.details ?? [] as d (d.label)}
          <div>
            <dt>{d.label}</dt>
            <dd>
              {#if isURL(d.value)}<a href={d.value} target="_blank" rel="noopener">{host(d.value)}</a>
              {:else if isPhone(d.label)}<a href="tel:{d.value.split(';')[0].replaceAll(' ', '')}">{d.value}</a>
              {:else if d.label === 'Email'}<a href="mailto:{d.value}">{d.value}</a>
              {:else}{d.value}{/if}
            </dd>
          </div>
        {/each}
      </dl>
    {/if}

    {#if place?.links?.length}
      <div class="links">
        {#each place.links as l (l.label)}
          <a class="chip" href={l.url} target="_blank" rel="noopener">{l.label} <Icon name="external" size={14} /></a>
        {/each}
      </div>
    {/if}

    {#if place?.photos?.length}
      <p class="credit">Photos from Wikimedia Commons · facts from Wikidata and OpenStreetMap</p>
    {/if}
  </div>
</aside>

<style>
  .panel {
    position: absolute;
    z-index: 5;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    left: 12px;
    top: 72px;
    max-height: calc(100% - 84px);
    width: 400px;
    border-radius: 16px;
  }
  .grip {
    display: none;
  }
  header {
    display: flex;
    gap: 8px;
    padding: 16px 12px 8px 18px;
  }
  .titles {
    flex: 1;
    min-width: 0;
  }
  h1 {
    margin: 0;
    font-size: 22px;
    line-height: 1.2;
    font-weight: 700;
    overflow-wrap: anywhere;
  }
  .sub {
    margin: 4px 0 0;
    color: var(--text);
    opacity: 0.85;
  }
  .ctx {
    margin: 2px 0 0;
    font-size: 13px;
    color: var(--muted);
  }
  .icon-btn {
    width: 36px;
    height: 36px;
    border-radius: 50%;
    background: var(--hover);
    flex: none;
  }
  .body {
    flex: 1;
    overflow-y: auto;
    padding: 4px 0 18px;
    overscroll-behavior: contain;
  }
  .photos {
    display: flex;
    gap: 6px;
    overflow-x: auto;
    padding: 6px 18px 10px;
    scroll-snap-type: x mandatory;
    scrollbar-width: none;
  }
  .photo {
    flex: none;
    width: 150px;
    height: 112px;
    border-radius: 10px;
    overflow: hidden;
    padding: 0;
    scroll-snap-align: start;
    background: var(--hover);
  }
  .photo:first-child {
    width: 220px;
  }
  .photo img {
    width: 100%;
    height: 100%;
    object-fit: cover;
    display: block;
  }
  .skeleton,
  .lines span {
    animation: pulse 1.2s ease-in-out infinite;
  }
  @keyframes pulse {
    50% {
      opacity: 0.45;
    }
  }
  .lines {
    display: grid;
    gap: 8px;
    padding: 8px 18px;
  }
  .lines span {
    height: 12px;
    border-radius: 6px;
    background: var(--hover);
  }
  .lines .short {
    width: 60%;
  }
  .actions,
  .links {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    padding: 4px 18px 10px;
  }
  .chip {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    height: 32px;
    padding: 0 12px;
    border-radius: 16px;
    background: var(--hover);
    color: var(--text);
    font-size: 13px;
    text-decoration: none;
  }
  .chip:hover {
    background: var(--hover-strong);
  }
  .mono {
    font-variant-numeric: tabular-nums;
  }
  .facts {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 1px;
    margin: 6px 18px 12px;
    border-radius: 12px;
    overflow: hidden;
    background: var(--line);
  }
  .fact {
    background: var(--card);
    padding: 9px 12px;
    min-width: 0;
  }
  .fact.wide {
    grid-column: span 2;
  }
  .facts dt,
  .details dt {
    font-size: 12px;
    color: var(--muted);
  }
  .note {
    font-size: 11px;
  }
  .facts dd {
    margin: 2px 0 0;
    font-weight: 600;
    overflow-wrap: anywhere;
  }
  .summary {
    padding: 0 18px 8px;
    line-height: 1.5;
  }
  .summary p {
    margin: 0 0 6px;
  }
  a {
    color: var(--accent);
  }
  .more {
    font-size: 14px;
  }
  .details {
    margin: 6px 18px 10px;
    display: grid;
    gap: 10px;
  }
  .details dd {
    margin: 2px 0 0;
    overflow-wrap: anywhere;
  }
  .error {
    margin: 4px 18px 10px;
    color: var(--danger);
  }
  .credit {
    margin: 8px 18px 0;
    font-size: 11px;
    color: var(--muted);
  }

  @media (max-width: 720px) {
    .panel {
      left: 0;
      right: 0;
      top: auto;
      bottom: 0;
      max-height: none;
      width: auto;
      height: 46vh;
      border-radius: 18px 18px 0 0;
      padding-bottom: env(safe-area-inset-bottom);
      transition: height 0.25s ease;
    }
    .panel.expanded {
      height: calc(100% - 72px - env(safe-area-inset-top));
    }
    .grip {
      display: flex;
      justify-content: center;
      padding: 8px 0 0;
      width: 100%;
      flex: none;
    }
    .grip span {
      width: 40px;
      height: 5px;
      border-radius: 3px;
      background: var(--muted);
      opacity: 0.6;
    }
    header {
      padding-top: 6px;
    }
  }
</style>
