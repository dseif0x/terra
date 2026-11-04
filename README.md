# terra

A map of the world on satellite imagery, in the browser (installable as a PWA).

- **Satellite** view: Esri World Imagery with English labels, roads, borders,
  points of interest and mountain peaks from [OpenFreeMap](https://openfreemap.org)
  on top; a plain **Map** view (OpenFreeMap Liberty); **3D terrain** and a globe.
- **Search** for places, addresses or coordinates (`47.53, 13.6`), biased to
  what is on screen ([Photon](https://photon.komoot.io)).
- **Click** a label, a point of interest or anywhere on the map: a panel shows
  photos (Wikimedia Commons), facts (population, area, elevation, founding,
  capital, languages, time zone, … from Wikidata), the English Wikipedia
  summary, OSM details (opening hours, phone, website, cuisine) and links to
  Google Maps, Apple Maps, OpenStreetMap, Wikipedia and Wikidata.

The frontend (Svelte 5 + MapLibre GL) loads tiles straight from the tile
servers. The Go backend does the lookups (Photon, Nominatim, Wikidata,
Wikipedia, Commons) with an in-memory cache (24 h) and per-host rate limits
(Nominatim: 1 request/s, as its usage policy asks). It has no login of its
own; run it behind one.

## Run

```sh
docker run -p 8080:8080 ghcr.io/dseif0x/terra:latest
```

| Variable     | Default    | |
|--------------|------------|-|
| `PORT`       | `8080`     | listen port |
| `STATIC_DIR` | `./static` | built frontend |

The backend needs outbound HTTPS to `photon.komoot.io`,
`nominatim.openstreetmap.org`, `www.wikidata.org`, `en.wikipedia.org` and
`commons.wikimedia.org`; browsers load tiles from `tiles.openfreemap.org`,
`server.arcgisonline.com` and `s3.amazonaws.com`.

## Develop

```sh
cd backend && go run .          # :8080
cd frontend && npm i && npm run dev   # proxies /api to :8080
```

## API

- `GET /api/search?q=&lat=&lon=&zoom=` – search results with OSM id and extent
- `GET /api/place?osm=R546245` – details of an OSM object
- `GET /api/place?name=&lat=&lon=&label=poi|place&subclass=` – a label clicked on the map
- `GET /api/place?lat=&lon=&zoom=` – whatever is at a point, at the map's level of detail

## Data and attribution

Imagery © Esri, Maxar, Earthstar Geographics and the GIS User Community.
Map data © OpenStreetMap contributors, tiles by OpenFreeMap / OpenMapTiles.
Terrain: Mapzen terrain tiles on AWS. Facts from Wikidata (CC0), summaries
from Wikipedia (CC BY-SA), photos from Wikimedia Commons (each under its own
license, credited in the viewer).
