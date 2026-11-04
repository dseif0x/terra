<script>
  import Icon from './Icon.svelte';

  let { photos, index = $bindable(), onclose } = $props();
  const ph = $derived(photos[index]);
  let startX = 0;

  const step = (d) => (index = (index + d + photos.length) % photos.length);

  function key(e) {
    if (e.key === 'Escape') onclose();
    else if (e.key === 'ArrowRight') step(1);
    else if (e.key === 'ArrowLeft') step(-1);
  }
</script>

<svelte:window onkeydown={key} />

<div
  class="lightbox"
  role="dialog"
  aria-modal="true"
  aria-label="Photo"
  tabindex="-1"
  ontouchstart={(e) => (startX = e.touches[0].clientX)}
  ontouchend={(e) => {
    const dx = e.changedTouches[0].clientX - startX;
    if (Math.abs(dx) > 50) step(dx < 0 ? 1 : -1);
  }}
>
  <button class="close" onclick={onclose} aria-label="Close"><Icon name="x" size={24} /></button>
  {#if photos.length > 1}
    <button class="nav prev" onclick={() => step(-1)} aria-label="Previous"><Icon name="left" size={28} /></button>
    <button class="nav next" onclick={() => step(1)} aria-label="Next"><Icon name="right" size={28} /></button>
  {/if}
  {#key ph.page}
    <img src={ph.full} alt={ph.title} referrerpolicy="no-referrer" />
  {/key}
  <footer>
    <div class="t">{ph.title}</div>
    <div class="c">
      {#if ph.credit}{ph.credit} · {/if}{ph.license || 'Wikimedia Commons'} ·
      <a href={ph.page} target="_blank" rel="noopener">Source</a>
      · {index + 1}/{photos.length}
    </div>
  </footer>
</div>

<style>
  .lightbox {
    position: fixed;
    inset: 0;
    z-index: 50;
    background: rgba(0, 0, 0, 0.94);
    display: flex;
    align-items: center;
    justify-content: center;
    color: #fff;
  }
  img {
    max-width: 100%;
    max-height: calc(100% - 110px);
    object-fit: contain;
  }
  .close {
    position: absolute;
    top: calc(12px + env(safe-area-inset-top));
    right: 12px;
    width: 44px;
    height: 44px;
    border-radius: 50%;
    background: rgba(255, 255, 255, 0.12);
    color: #fff;
  }
  .nav {
    position: absolute;
    top: 50%;
    transform: translateY(-50%);
    width: 48px;
    height: 48px;
    border-radius: 50%;
    background: rgba(255, 255, 255, 0.12);
    color: #fff;
  }
  .prev {
    left: 12px;
  }
  .next {
    right: 12px;
  }
  footer {
    position: absolute;
    left: 0;
    right: 0;
    bottom: 0;
    padding: 12px 16px calc(14px + env(safe-area-inset-bottom));
    text-align: center;
  }
  .t {
    font-weight: 600;
  }
  .c {
    font-size: 12px;
    opacity: 0.75;
    margin-top: 3px;
  }
  a {
    color: #9cc8ff;
  }
  @media (max-width: 720px) {
    .nav {
      display: none;
    }
  }
</style>
