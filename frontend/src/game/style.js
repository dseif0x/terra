// The game's map: a blank globe with country outlines and no names.
// Countries take their colour from the feature state "c" (feedback and
// the progress map), else the plain land colour.

export const COLORS = {
  ocean: '#0e2236',
  land: '#3d4b5d',
  other: '#323d4b',
  border: 'rgba(200, 214, 230, 0.45)',
  hover: '#52647a',
  wrong: '#e5484d',
  right: '#30a46c',
  missed: '#f5c518',
};

export function gameStyle(features) {
  return {
    version: 8,
    projection: { type: 'globe' },
    sky: {
      'sky-color': '#081420',
      'horizon-color': '#24527e',
      'fog-color': '#0b1a29',
      'atmosphere-blend': ['interpolate', ['linear'], ['zoom'], 0, 1, 5, 0.4, 8, 0],
    },
    sources: {
      countries: {
        type: 'geojson',
        data: features,
        attribution:
          'Outlines: <a href="https://www.naturalearthdata.com/" target="_blank" rel="noopener">Natural Earth</a> · data: <a href="https://github.com/mledoze/countries" target="_blank" rel="noopener">mledoze/countries</a> (ODbL) · flags: <a href="https://github.com/lipis/flag-icons" target="_blank" rel="noopener">flag-icons</a>',
      },
    },
    layers: [
      { id: 'ocean', type: 'background', paint: { 'background-color': COLORS.ocean } },
      {
        id: 'game-fill',
        type: 'fill',
        source: 'countries',
        paint: {
          'fill-color': [
            'case',
            ['!=', ['feature-state', 'c'], null],
            ['to-color', ['feature-state', 'c']],
            ['boolean', ['feature-state', 'hover'], false],
            COLORS.hover,
            ['==', ['get', 't'], 1],
            COLORS.land,
            COLORS.other,
          ],
          'fill-antialias': true,
        },
      },
      {
        id: 'game-border',
        type: 'line',
        source: 'countries',
        paint: { 'line-color': COLORS.border, 'line-width': ['interpolate', ['linear'], ['zoom'], 1, 0.4, 6, 1.2] },
      },
      {
        // outline of a country being shown (found or revealed)
        id: 'game-mark',
        type: 'line',
        source: 'countries',
        filter: ['==', ['id'], -1],
        paint: { 'line-color': '#ffffff', 'line-width': 2.2 },
      },
    ],
  };
}
