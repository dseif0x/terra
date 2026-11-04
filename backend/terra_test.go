package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestFormatInt(t *testing.T) {
	for in, want := range map[float64]string{0: "0", 741: "741", 1234: "1,234", 2028289: "2,028,289", -4500.4: "-4,500"} {
		if got := formatInt(in); got != want {
			t.Errorf("formatInt(%v) = %q, want %q", in, got, want)
		}
	}
}

func TestFormatArea(t *testing.T) {
	for in, want := range map[float64]string{349790: "349,790 km²", 59.79: "59.8 km²", 0.5: "50 ha", 0.002: "2,000 m²"} {
		if got := formatArea(in); got != want {
			t.Errorf("formatArea(%v) = %q, want %q", in, got, want)
		}
	}
}

func TestWdTime(t *testing.T) {
	cases := []struct {
		t    wdTime
		want string
	}{
		{wdTime{"+1137-00-00T00:00:00Z", 9}, "1137"},
		{wdTime{"-0100-00-00T00:00:00Z", 7}, "1st century BC"},
		{wdTime{"+1850-00-00T00:00:00Z", 7}, "19th century"},
		{wdTime{"+1990-00-00T00:00:00Z", 8}, "1990s"},
		{wdTime{"+2011-03-04T00:00:00Z", 11}, "2011"},
	}
	for _, c := range cases {
		if got := c.t.String(); got != c.want {
			t.Errorf("%v = %q, want %q", c.t, got, c.want)
		}
	}
}

func TestKindLabel(t *testing.T) {
	cases := [][4]string{
		{"amenity", "restaurant", "house", "Restaurant"},
		{"place", "city", "city", "City"},
		{"boundary", "administrative", "other", "Area"},
		{"natural", "peak", "", "Mountain peak"},
		{"highway", "residential", "street", "Road"},
		{"building", "yes", "", "Building"},
		{"highway", "platform", "", "Bus stop"},
	}
	for _, c := range cases {
		if got := kindLabel(c[0], c[1], c[2]); got != c[3] {
			t.Errorf("kindLabel(%q, %q, %q) = %q, want %q", c[0], c[1], c[2], got, c[3])
		}
	}
}

func TestReverseZoom(t *testing.T) {
	for z, want := range map[float64]int{1.5: 3, 5: 5, 8: 8, 11: 10, 13: 14, 16: 17, 19: 18} {
		if got := reverseZoom(z); got != want {
			t.Errorf("reverseZoom(%v) = %d, want %d", z, got, want)
		}
	}
}

func claim(rank string, value any, quals map[string][]any) wdClaim {
	snak := func(v any) wdSnak {
		var s wdSnak
		s.SnakType = "value"
		s.DataValue.Value, _ = json.Marshal(v)
		return s
	}
	c := wdClaim{MainSnak: snak(value), Rank: rank, Qualifiers: map[string][]wdSnak{}}
	for p, vs := range quals {
		for _, v := range vs {
			c.Qualifiers[p] = append(c.Qualifiers[p], snak(v))
		}
	}
	return c
}

func TestFactsPopulationAndArea(t *testing.T) {
	e := &wdEntity{Claims: map[string][]wdClaim{
		"P1082": {
			claim("normal", wdQuantity{"+800", "1"}, map[string][]any{"P585": {wdTime{"+2001-00-00T00:00:00Z", 9}}}),
			claim("normal", wdQuantity{"+741", "1"}, map[string][]any{"P585": {wdTime{"+2024-01-01T00:00:00Z", 11}}}),
			claim("deprecated", wdQuantity{"+5", "1"}, nil),
		},
		"P2046": {claim("normal", wdQuantity{"+59.79", "http://www.wikidata.org/entity/Q712226"}, nil)},
		"P2044": {claim("normal", wdQuantity{"+1676", "http://www.wikidata.org/entity/Q3710"}, nil)},
		"P856":  {claim("normal", "https://example.org/", nil)},
	}}
	s := &server{up: newUpstream()}
	got := s.facts(context.Background(), e)
	want := []Fact{
		{"Population", "741", "2024"},
		{"Area", "59.8 km²", ""},
		{"Density", "12 / km²", ""},
		{"Elevation", "511 m", ""},
		{"Website", "https://example.org/", ""},
	}
	if len(got) != len(want) {
		t.Fatalf("facts = %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("fact %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// photonURL is the request findNamed makes, so a test can put the answer
// into the cache.
func photonURL(q string, lat, lon float64) string {
	v := url.Values{"q": {q}, "lang": {"en"}, "limit": {"10"}, "lat": {fmtFloat(lat)}, "lon": {fmtFloat(lon)}, "zoom": {"14"}}
	return "https://photon.komoot.io/api/?" + v.Encode()
}

func TestFindNamed(t *testing.T) {
	s := &server{up: newUpstream()}
	lat, lon := 48.20849, 16.37208
	s.up.store(photonURL("Stephansdom", lat, lon), []byte(`{"features":[
		{"geometry":{"coordinates":[16.37313,48.20849]},"properties":{"osm_type":"W","osm_id":212496461,"osm_key":"amenity","osm_value":"place_of_worship","name":"Saint Stephen's Cathedral"}},
		{"geometry":{"coordinates":[16.37296,48.20828]},"properties":{"osm_type":"N","osm_id":4543089891,"osm_key":"tourism","osm_value":"attraction","name":"Stephansdom Crypt"}},
		{"geometry":{"coordinates":[6.1757,49.1202]},"properties":{"osm_type":"W","osm_id":32694789,"osm_key":"amenity","osm_value":"place_of_worship","name":"Stephansdom"}}
	]}`))
	ctx := context.Background()
	// the far-away exact name loses; the right class nearby wins
	if got, err := s.findNamed(ctx, "Stephansdom", "place_of_worship", lat, lon, 0.5, 0.2); err != nil || got != "W212496461" {
		t.Errorf("with subclass: %q, %v", got, err)
	}
	// without a class, the nearest hit
	if got, _ := s.findNamed(ctx, "Stephansdom", "", lat, lon, 0.5, 0.2); got != "N4543089891" {
		t.Errorf("without subclass: %q", got)
	}
	// a far exact name is fine for town labels
	if got, _ := s.findNamed(ctx, "Stephansdom", "", lat, lon, 1000, 0); got != "W32694789" {
		t.Errorf("place label: %q", got)
	}
}

func TestPlaceValidation(t *testing.T) {
	s := &server{up: newUpstream()}
	h := s.routes()
	for _, q := range []string{"", "osm=X1", "osm=N1;drop", "lat=91&lon=0", "lat=1"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/place?"+q, nil))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%q: status %d", q, rec.Code)
		}
	}
}

func TestCacheEviction(t *testing.T) {
	up := newUpstream()
	big := make([]byte, cacheMaxBytes/8)
	for i := range 10 {
		up.store(string(rune('a'+i)), big)
	}
	if up.bytes > cacheMaxBytes {
		t.Fatalf("cache holds %d bytes", up.bytes)
	}
	if _, ok := up.cached("a"); ok {
		t.Error("oldest entry not evicted")
	}
	if _, ok := up.cached("j"); !ok {
		t.Error("newest entry missing")
	}
}
