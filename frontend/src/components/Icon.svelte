<script module>
  import Search from '@lucide/svelte/icons/search';
  import X from '@lucide/svelte/icons/x';
  import Layers from '@lucide/svelte/icons/layers';
  import Tag from '@lucide/svelte/icons/tag';
  import Mountain from '@lucide/svelte/icons/mountain';
  import Globe from '@lucide/svelte/icons/globe';
  import MapPin from '@lucide/svelte/icons/map-pin';
  import ExternalLink from '@lucide/svelte/icons/external-link';
  import Copy from '@lucide/svelte/icons/copy';
  import Check from '@lucide/svelte/icons/check';
  import ChevronLeft from '@lucide/svelte/icons/chevron-left';
  import ChevronRight from '@lucide/svelte/icons/chevron-right';
  import Loader from '@lucide/svelte/icons/loader-circle';
  import Satellite from '@lucide/svelte/icons/satellite';
  import MapIcon from '@lucide/svelte/icons/map';
  import Share from '@lucide/svelte/icons/share-2';

  const ICONS = {
    search: Search,
    x: X,
    layers: Layers,
    tag: Tag,
    mountain: Mountain,
    globe: Globe,
    pin: MapPin,
    external: ExternalLink,
    copy: Copy,
    check: Check,
    left: ChevronLeft,
    right: ChevronRight,
    loader: Loader,
    satellite: Satellite,
    map: MapIcon,
    share: Share,
  };
</script>

<script>
  let { name, size = 20 } = $props();
  const C = $derived(ICONS[name]);
</script>

{#if C}<C {size} strokeWidth={2} aria-hidden="true" />{/if}
