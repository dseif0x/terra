<script>
  import { onMount } from 'svelte';
  import * as maplibregl from 'maplibre-gl';
  import 'maplibre-gl/dist/maplibre-gl.css';
  // MapLibre looks for its worker next to its own module, which bundling
  // breaks: hand it the worker, bundled separately by Vite.
  import workerUrl from 'maplibre-gl/dist/maplibre-gl-worker.mjs?worker&url';
  import SearchBar from './components/SearchBar.svelte';
  import PlacePanel from './components/PlacePanel.svelte';
  import Lightbox from './components/Lightbox.svelte';
  import Icon from './components/Icon.svelte';
  import Game from './game/Game.svelte';
  import { loadBase, satelliteStyle, mapStyle, overlayLayers, CLICKABLE } from './lib/style.js';
  import { place as fetchPlace } from './lib/api.js';

  maplibregl.setWorkerUrl(workerUrl);

  const PREFS = 'terra.prefs';
  function loadPrefs() {
    try {
      return JSON.parse(localStorage.getItem(PREFS)) || {};
    } catch {
      return {};
    }
  }
  const prefs = loadPrefs();

  let mapEl;
  let map = $state.raw(null);
  let marker;
  let baseStyle;
  let clickable = [];
  let ready = false; // the map's first load event has fired
  let mode = $state(prefs.mode === 'map' ? 'map' : 'satellite');
  let labels = $state(prefs.labels !== false);
  let relief = $state(false);
  let fatal = $state('');
  // Explore (the map) or Game (find countries on a blank globe)
  const query = new URLSearchParams(location.search);
  const duelParam = query.get('duel');
  let app = $state(query.has('game') || duelParam ? 'game' : 'explore');
  let mapReady = $state(false);

  // the selected place: preview (known at once), then the details
  let sel = $state(null);
  let photo = $state(null);
  let ctrl;

  $effect(() => {
    try {
      localStorage.setItem(PREFS, JSON.stringify({ mode, labels }));
    } catch {}
  });

  const desktop = () => window.matchMedia('(min-width: 721px)').matches;
  const padding = () =>
    desktop()
      ? { top: 90, bottom: 40, left: sel ? 440 : 40, right: 70 }
      : { top: 90, bottom: sel ? Math.round(innerHeight * 0.48) : 40, left: 30, right: 30 };

  onMount(() => {
    let alive = true;
    (async () => {
      try {
        baseStyle = await loadBase();
      } catch (e) {
        fatal = 'The map could not load: ' + e.message;
        return;
      }
      if (!alive) return;
      const hadView = location.hash.includes('map='); // the map writes its hash at once
      map = new maplibregl.Map({
        container: mapEl,
        style: buildStyle(),
        hash: 'map',
        center: [12, 35],
        zoom: 1.7,
        maxZoom: 20.5,
        maxPitch: 80,
        attributionControl: { compact: true },
      });
      window.terraMap = map; // for debugging from the console
      mapReady = true;
      map.addControl(new maplibregl.NavigationControl({ visualizePitch: true }), 'bottom-right');
      map.addControl(
        new maplibregl.GeolocateControl({
          positionOptions: { enableHighAccuracy: true },
          trackUserLocation: true,
          fitBoundsOptions: { maxZoom: 15 },
        }),
        'bottom-right',
      );
      map.addControl(new maplibregl.ScaleControl({ unit: 'metric' }), 'bottom-left');
      map.once('load', () => {
        ready = true;
        // the compact attribution starts expanded and covers the map on phones
        mapEl.querySelector('.maplibregl-ctrl-attrib')?.classList.remove('maplibregl-compact-show');
        mapEl.querySelector('.maplibregl-ctrl-attrib')?.removeAttribute('open');
      });
      map.on('style.load', styleLoaded);
      map.on('click', onClick);
      map.on('mousemove', (e) => {
        if (app === 'game') return;
        const hit = clickable.length && map.queryRenderedFeatures(box(e.point, 4), { layers: clickable }).length;
        map.getCanvas().style.cursor = hit ? 'pointer' : '';
      });
      const p = new URLSearchParams(location.search).get('p');
      if (p && app === 'explore') open({}, { osm: p }, !hadView);
    })();
    return () => {
      alive = false;
      map?.remove();
    };
  });

  function enterGame() {
    close();
    app = 'game';
    history.replaceState(null, '', '?game' + location.hash);
  }

  function exitGame() {
    app = 'explore';
    map.setStyle(buildStyle(), { diff: false });
    history.replaceState(null, '', location.pathname + location.hash);
  }

  function buildStyle() {
    return mode === 'map' ? mapStyle(baseStyle) : satelliteStyle(baseStyle);
  }

  function styleLoaded() {
    clickable = map
      .getStyle()
      .layers.filter((l) => l.type === 'symbol' && CLICKABLE[l['source-layer']])
      .map((l) => l.id);
    applyLabels();
    applyRelief(false);
  }

  function applyLabels() {
    if (!map?.isStyleLoaded()) return;
    const v = labels || mode === 'map' ? 'visible' : 'none';
    for (const id of overlayLayers(map)) map.setLayoutProperty(id, 'visibility', v);
  }

  function applyRelief(animate) {
    if (!map?.getSource('terrain')) return;
    map.setTerrain(relief ? { source: 'terrain', exaggeration: 1.3 } : null);
    if (animate) map.easeTo({ pitch: relief ? 60 : 0, duration: 800 });
  }

  function setMode(m) {
    if (m === mode) return;
    mode = m;
    map?.setStyle(buildStyle(), { diff: false });
  }

  const box = (p, r) => [
    [p.x - r, p.y - r],
    [p.x + r, p.y + r],
  ];

  const pretty = (s) => (s ? (s[0].toUpperCase() + s.slice(1)).replaceAll('_', ' ') : '');

  function onClick(e) {
    if (app === 'game') return;
    const hits = clickable.length
      ? map.queryRenderedFeatures(box(e.point, 8), { layers: clickable }).filter((f) => f.properties.name)
      : [];
    if (hits.length) {
      const f = hits[0];
      const pr = f.properties;
      const [lon, lat] = f.geometry.type === 'Point' ? f.geometry.coordinates : [e.lngLat.lng, e.lngLat.lat];
      const name = pr['name:en'] || pr.name;
      const subclass = f.sourceLayer === 'mountain_peak' ? 'peak' : pr.subclass || pr.class;
      open(
        { name, kind: pretty(pr.subclass || pr.class), lat, lon },
        { name, lat: lat.toFixed(6), lon: lon.toFixed(6), label: CLICKABLE[f.sourceLayer], subclass },
      );
    } else {
      const { lat, lng } = e.lngLat;
      open({ lat, lon: lng }, { lat: lat.toFixed(6), lon: lng.toFixed(6), zoom: map.getZoom().toFixed(1) });
    }
  }

  async function open(preview, params, fit = false) {
    ctrl?.abort();
    ctrl = new AbortController();
    const signal = ctrl.signal;
    sel = { preview, place: null, loading: true, error: '' };
    photo = null;
    if (preview.lat != null) setMarker(preview.lon, preview.lat);
    try {
      const p = await fetchPlace(params, signal);
      if (signal.aborted) return;
      sel = { ...sel, place: p, loading: false };
      if (preview.lat == null || params.osm) setMarker(p.lon, p.lat);
      if (fit) ready ? focus() : map.once('load', focus);
      if (p.osm) history.replaceState(null, '', `?p=${p.osm}${location.hash}`);
    } catch (e) {
      if (signal.aborted || e.name === 'AbortError') return;
      sel = { ...sel, loading: false, error: e.message === 'nothing found here' ? 'Nothing named here.' : e.message };
    }
  }

  function setMarker(lon, lat) {
    if (!marker) marker = new maplibregl.Marker({ color: '#ff453a' });
    marker.setLngLat([lon, lat]).addTo(map);
  }

  function close() {
    ctrl?.abort();
    sel = null;
    marker?.remove();
    history.replaceState(null, '', location.pathname + location.hash);
  }

  function focus() {
    const p = sel?.place;
    if (!p) return;
    if (p.bbox) map.fitBounds(p.bbox, { padding: padding(), maxZoom: 17, duration: 1600 });
    else map.flyTo({ center: [p.lon, p.lat], zoom: Math.max(map.getZoom(), 16), padding: padding(), duration: 1600 });
  }

  function onpick(r) {
    if (r.coords) {
      open({ lat: r.lat, lon: r.lon }, { lat: r.lat, lon: r.lon, zoom: 16 });
      map.flyTo({ center: [r.lon, r.lat], zoom: 15, duration: 1600 });
      return;
    }
    const preview = { name: r.name, kind: r.kind };
    if (r.osm) open(preview, { osm: r.osm });
    else open({ ...preview, lat: r.lat, lon: r.lon }, { lat: r.lat, lon: r.lon, zoom: 18 });
    // move now, from the search result, rather than after the details
    setMarker(r.lon, r.lat);
    if (r.extent) map.fitBounds(r.extent, { padding: padding(), maxZoom: 16, duration: 1800 });
    else map.flyTo({ center: [r.lon, r.lat], zoom: 16, padding: padding(), duration: 1800 });
  }

  const bias = () => (map ? { center: map.getCenter(), zoom: map.getZoom() } : {});
</script>

<div class="map" class:game={app === 'game'} bind:this={mapEl}></div>

{#if fatal}
  <div class="fatal glass">{fatal}</div>
{/if}

{#if app === 'game'}
  {#if mapReady}<Game {map} {duelParam} onexit={exitGame} />{/if}
{:else}
<div class="top">
  <SearchBar {onpick} {bias} />
</div>

<div class="tools">
  <div class="group glass">
    <button
      class:on={mode === 'satellite'}
      onclick={() => setMode('satellite')}
      title="Satellite"
      aria-label="Satellite"
      aria-pressed={mode === 'satellite'}><Icon name="satellite" /></button
    >
    <button class:on={mode === 'map'} onclick={() => setMode('map')} title="Map" aria-label="Map" aria-pressed={mode === 'map'}
      ><Icon name="map" /></button
    >
  </div>
  <div class="group glass">
    {#if mode === 'satellite'}
      <button
        class:on={labels}
        onclick={() => {
          labels = !labels;
          applyLabels();
        }}
        title="Labels"
        aria-label="Labels"
        aria-pressed={labels}><Icon name="tag" /></button
      >
    {/if}
    <button
      class:on={relief}
      onclick={() => {
        relief = !relief;
        applyRelief(true);
      }}
      title="3D terrain"
      aria-label="3D terrain"
      aria-pressed={relief}><Icon name="mountain" /></button
    >
  </div>
  <div class="group glass">
    <button onclick={enterGame} title="Game: find the countries" aria-label="Game"><Icon name="gamepad" /></button>
  </div>
</div>

{#if sel}
  <PlacePanel
    place={sel.place}
    preview={sel.preview}
    loading={sel.loading}
    error={sel.error}
    onclose={close}
    onfocus={focus}
    onphoto={(i) => (photo = i)}
  />
{/if}

{#if photo !== null && sel?.place?.photos?.length}
  <Lightbox photos={sel.place.photos} bind:index={photo} onclose={() => (photo = null)} />
{/if}
{/if}

<style>
  .map {
    position: absolute;
    inset: 0;
  }
  /* Game mode keeps the globe bare; the data credits are in its help */
  .map.game :global(.maplibregl-ctrl-geolocate),
  .map.game :global(.maplibregl-ctrl-scale),
  .map.game :global(.maplibregl-ctrl-attrib) {
    display: none;
  }
  .map.game:global(.game-menu) :global(.maplibregl-ctrl-group) {
    display: none;
  }
  .top {
    position: absolute;
    z-index: 10;
    top: calc(12px + env(safe-area-inset-top));
    left: 12px;
    width: 400px;
  }
  .tools {
    position: absolute;
    z-index: 4;
    top: calc(12px + env(safe-area-inset-top));
    right: 12px;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .group {
    display: flex;
    flex-direction: column;
    border-radius: 12px;
    overflow: hidden;
  }
  .group button {
    width: 44px;
    height: 44px;
    color: var(--muted);
  }
  .group button + button {
    border-top: 1px solid var(--line);
  }
  .group button.on {
    color: var(--accent);
  }
  .fatal {
    position: absolute;
    z-index: 20;
    top: 50%;
    left: 50%;
    transform: translate(-50%, -50%);
    padding: 16px 20px;
    border-radius: 12px;
    max-width: 80vw;
  }
  @media (max-width: 720px) {
    .top {
      right: 12px;
      width: auto;
    }
    .tools {
      top: calc(72px + env(safe-area-inset-top));
    }
  }
</style>
