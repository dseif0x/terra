package main

import (
	"context"
	"math"
	"net/url"
	"strconv"
	"strings"
)

// SearchResult is one place from a search, enough to show it in the list
// and to move the map there.
type SearchResult struct {
	Name     string    `json:"name"`
	Kind     string    `json:"kind"`
	Context  string    `json:"context,omitempty"`
	Lat      float64   `json:"lat"`
	Lon      float64   `json:"lon"`
	OSM      string    `json:"osm,omitempty"`    // N123, W123 or R123
	Extent   []float64 `json:"extent,omitempty"` // west, south, east, north
	osmValue string
}

type photonResponse struct {
	Features []struct {
		Geometry struct {
			Coordinates []float64 `json:"coordinates"`
		} `json:"geometry"`
		Properties struct {
			OSMType     string    `json:"osm_type"`
			OSMID       int64     `json:"osm_id"`
			OSMKey      string    `json:"osm_key"`
			OSMValue    string    `json:"osm_value"`
			Type        string    `json:"type"`
			Name        string    `json:"name"`
			Street      string    `json:"street"`
			HouseNumber string    `json:"housenumber"`
			City        string    `json:"city"`
			District    string    `json:"district"`
			County      string    `json:"county"`
			State       string    `json:"state"`
			Country     string    `json:"country"`
			Extent      []float64 `json:"extent"` // minLon, maxLat, maxLon, minLat
		} `json:"properties"`
	} `json:"features"`
}

// search asks Photon (OSM data, made for search-as-you-type). With a
// position the results are biased towards it.
func (s *server) search(ctx context.Context, q string, lat, lon *float64, zoom float64, limit int) ([]SearchResult, error) {
	v := url.Values{"q": {q}, "lang": {"en"}, "limit": {strconv.Itoa(limit)}}
	if lat != nil && lon != nil {
		v.Set("lat", fmtFloat(*lat))
		v.Set("lon", fmtFloat(*lon))
		if zoom > 0 {
			v.Set("zoom", strconv.Itoa(int(math.Round(zoom))))
		}
	}
	var pr photonResponse
	if err := s.up.getJSON(ctx, "https://photon.komoot.io/api/?"+v.Encode(), &pr); err != nil {
		return nil, err
	}
	out := make([]SearchResult, 0, len(pr.Features))
	seen := map[string]bool{}
	for _, f := range pr.Features {
		p := f.Properties
		if len(f.Geometry.Coordinates) < 2 {
			continue
		}
		r := SearchResult{
			Lon:      f.Geometry.Coordinates[0],
			Lat:      f.Geometry.Coordinates[1],
			osmValue: p.OSMValue,
		}
		if p.OSMType != "" && p.OSMID != 0 {
			r.OSM = p.OSMType + strconv.FormatInt(p.OSMID, 10)
		}
		if seen[r.OSM] && r.OSM != "" {
			continue
		}
		seen[r.OSM] = true
		r.Name = p.Name
		street := strings.TrimSpace(p.Street + " " + p.HouseNumber)
		if r.Name == "" {
			r.Name = street
			street = ""
		}
		if r.Name == "" {
			continue
		}
		r.Kind = kindLabel(p.OSMKey, p.OSMValue, p.Type)
		r.Context = joinNonEmpty(", ", r.Name, street, p.District, p.City, p.County, p.State, p.Country)
		if len(p.Extent) == 4 {
			r.Extent = []float64{p.Extent[0], p.Extent[3], p.Extent[2], p.Extent[1]}
		}
		out = append(out, r)
	}
	return out, nil
}

// kindLabel turns OSM's key/value into a short English label: "Restaurant",
// "City", "Mountain peak".
func kindLabel(key, value, typ string) string {
	switch {
	case key == "place" || (key == "boundary" && value == "administrative"):
		if typ != "" && typ != "other" {
			value = typ
		} else if key == "boundary" {
			value = "area"
		}
	case key == "natural" && value == "peak":
		value = "mountain peak"
	case key == "highway":
		switch value {
		case "bus_stop", "platform":
			value = "bus stop"
		case "pedestrian":
			value = "pedestrian street"
		case "footway", "path", "cycleway", "steps", "track":
		default:
			value = "road"
		}
	case value == "yes":
		value = key
	}
	return capitalize(strings.ReplaceAll(value, "_", " "))
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func joinNonEmpty(sep string, first string, parts ...string) string {
	var out []string
	seen := map[string]bool{first: true}
	for _, p := range parts {
		if p != "" && !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	return strings.Join(out, sep)
}

func fmtFloat(f float64) string { return strconv.FormatFloat(f, 'f', 6, 64) }
