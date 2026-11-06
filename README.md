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
- **Game** mode (the controller button, or `/?game`): find countries on a
  blank globe. Three tries per country (1,000 / 650 / 350 points, then up to
  150 for the closest miss), with the distance and direction after a wrong tap.
  - **Daily**: five countries a day from easy to hard, the same for everyone,
    with a streak, share text and past days to play.
  - **Blitz**: after the daily, five more with one tap and 20 seconds each.
  - **Duels**: a challenge link to your daily (`/?duel=…`); a friend plays the
    same five and sees who won.
  - **Learn**: spaced repetition by region (or the countries independent since
    1990), with a progress map.
  - **Practice**: unlimited, by name and flag, flag only or shape, by region
    and difficulty.

  Progress is kept in the browser (localStorage). The 197 countries are the UN
  members plus the Vatican, Palestine, Kosovo and Taiwan.

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
license, credited in the viewer). Game: outlines from Natural Earth (public
domain, via world-atlas), country facts from
[mledoze/countries](https://github.com/mledoze/countries) (ODbL), flags from
[flag-icons](https://github.com/lipis/flag-icons) (MIT); built into
`frontend/public/game/` by `npm run game-data` (part of `npm run build`).
