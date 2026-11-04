// terra is a map of the world on satellite imagery: search, points of
// interest and, for any place clicked, photos and facts. The frontend draws
// the map straight from public tile servers; this backend does the lookups
// (Photon, Nominatim, Wikidata, Wikipedia, Wikimedia Commons) with a cache
// and the rate limits their usage policies ask for. It has no login of its
// own: it runs behind the cluster's forward-auth login.
package main

import (
	"encoding/json"
	"errors"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type server struct {
	up     *upstream
	static string
}

func main() {
	s := &server{up: newUpstream(), static: envOr("STATIC_DIR", "./static")}
	addr := ":" + envOr("PORT", "8080")
	log.Printf("terra listening on %s", addr)
	srv := &http.Server{Addr: addr, Handler: s.routes(), ReadHeaderTimeout: 10 * time.Second}
	log.Fatal(srv.ListenAndServe())
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) })
	mux.HandleFunc("GET /api/search", s.handleSearch)
	mux.HandleFunc("GET /api/place", s.handlePlace)
	mux.HandleFunc("GET /", s.handleStatic)
	return mux
}

func (s *server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" || len(q) > 200 {
		writeJSON(w, http.StatusOK, []SearchResult{})
		return
	}
	lat, lon := floatParam(r, "lat"), floatParam(r, "lon")
	zoom := 0.0
	if z := floatParam(r, "zoom"); z != nil {
		zoom = *z
	}
	rs, err := s.search(r.Context(), q, lat, lon, zoom, 8)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rs)
}

// handlePlace looks a place up by OSM id (?osm=R546245), by a label clicked
// on the map (?name=&lat=&lon=&label=poi|place&subclass=, from the vector tiles), or by a point (?lat=&lon=
// &zoom=, the map's zoom).
func (s *server) handlePlace(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	osm := q.Get("osm")
	lat, lon := floatParam(r, "lat"), floatParam(r, "lon")
	if osm == "" && (lat == nil || lon == nil || math.Abs(*lat) > 90 || math.Abs(*lon) > 180) {
		http.Error(w, "osm or lat/lon required", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	if osm == "" {
		if name := strings.TrimSpace(q.Get("name")); name != "" && len(name) <= 200 {
			maxKm, nearKm := 0.5, 0.2
			if q.Get("label") == "place" {
				// labels of towns and regions sit away from their node
				maxKm, nearKm = 30, 3
			}
			if ref, err := s.findNamed(ctx, name, q.Get("subclass"), *lat, *lon, maxKm, nearKm); err == nil {
				osm = ref
			}
		}
	}
	var np *nominatimPlace
	var err error
	if osm != "" {
		if !osmRefRe.MatchString(osm) {
			http.Error(w, "invalid osm id", http.StatusBadRequest)
			return
		}
		np, err = s.lookup(ctx, osm)
	} else {
		zoom := 18
		if z := floatParam(r, "zoom"); z != nil {
			zoom = reverseZoom(*z)
		}
		np, err = s.reverse(ctx, *lat, *lon, zoom)
	}
	if err != nil {
		writeError(w, err)
		return
	}
	p, err := s.place(ctx, np)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// reverseZoom maps the map's zoom to the detail Nominatim should answer
// with: zoomed out, a click means the country or region, not a house.
func reverseZoom(z float64) int {
	switch {
	case z < 4:
		return 3
	case z < 6:
		return 5
	case z < 9:
		return 8
	case z < 12:
		return 10
	case z < 15:
		return 14
	case z < 17:
		return 17
	}
	return 18
}

func floatParam(r *http.Request, k string) *float64 {
	v, err := strconv.ParseFloat(r.URL.Query().Get(k), 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return nil
	}
	return &v
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, err error) {
	if errors.Is(err, errNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "nothing found here"})
		return
	}
	log.Printf("upstream: %v", err)
	writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
}

// handleStatic serves the built frontend; unknown paths get index.html.
func (s *server) handleStatic(w http.ResponseWriter, r *http.Request) {
	p := filepath.Join(s.static, filepath.FromSlash(filepath.Clean("/"+r.URL.Path)))
	if st, err := os.Stat(p); err != nil || st.IsDir() {
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, filepath.Join(s.static, "index.html"))
		return
	}
	if strings.HasPrefix(r.URL.Path, "/assets/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}
	http.ServeFile(w, r, p)
}
