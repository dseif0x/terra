<script>
  // A country's shape on its own: its main landmass and the parts near it,
  // in an equal-area projection centred on it, so the outline is not
  // stretched the way it is on a flat map.
  import { geoAzimuthalEqualArea, geoPath, geoCentroid } from 'd3-geo';
  import { mainParts } from './geo.js';

  let { feature, width = 220, height = 150, color = 'currentColor' } = $props();

  const d = $derived.by(() => {
    const shape = { type: 'Feature', geometry: { type: 'MultiPolygon', coordinates: mainParts(feature) } };
    const [lon, lat] = geoCentroid(shape);
    const proj = geoAzimuthalEqualArea()
      .rotate([-lon, -lat])
      .fitExtent(
        [
          [6, 6],
          [width - 6, height - 6],
        ],
        shape,
      );
    return geoPath(proj)(shape);
  });
</script>

<svg viewBox="0 0 {width} {height}" {width} {height} aria-label="Country shape" role="img">
  <path {d} fill={color} />
</svg>

<style>
  svg {
    display: block;
    max-width: 100%;
    height: auto;
  }
</style>
