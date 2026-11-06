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
  import Gamepad from '@lucide/svelte/icons/gamepad-2';
  import Volume from '@lucide/svelte/icons/volume-2';
  import VolumeX from '@lucide/svelte/icons/volume-x';
  import ChartColumn from '@lucide/svelte/icons/chart-column';
  import Swords from '@lucide/svelte/icons/swords';
  import CalendarDays from '@lucide/svelte/icons/calendar-days';
  import Zap from '@lucide/svelte/icons/zap';
  import GraduationCap from '@lucide/svelte/icons/graduation-cap';
  import Dumbbell from '@lucide/svelte/icons/dumbbell';
  import Flame from '@lucide/svelte/icons/flame';
  import SkipForward from '@lucide/svelte/icons/skip-forward';

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
    gamepad: Gamepad,
    volume: Volume,
    mute: VolumeX,
    stats: ChartColumn,
    swords: Swords,
    calendar: CalendarDays,
    zap: Zap,
    learn: GraduationCap,
    practice: Dumbbell,
    flame: Flame,
    skip: SkipForward,
  };
</script>

<script>
  let { name, size = 20 } = $props();
  const C = $derived(ICONS[name]);
</script>

{#if C}<C {size} strokeWidth={2} aria-hidden="true" />{/if}
