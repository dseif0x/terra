package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"unicode"
)

// Place is what the details panel shows.
type Place struct {
	Name        string    `json:"name"`
	Kind        string    `json:"kind,omitempty"`
	Description string    `json:"description,omitempty"`
	Context     string    `json:"context,omitempty"`
	Address     string    `json:"address,omitempty"`
	Lat         float64   `json:"lat"`
	Lon         float64   `json:"lon"`
	OSM         string    `json:"osm,omitempty"`
	BBox        []float64 `json:"bbox,omitempty"` // west, south, east, north
	Facts       []Fact    `json:"facts,omitempty"`
	Details     []Fact    `json:"details,omitempty"`
	Summary     string    `json:"summary,omitempty"`
	Photos      []Photo   `json:"photos,omitempty"`
	Links       []Link    `json:"links,omitempty"`
}

type Fact struct {
	Label string `json:"label"`
	Value string `json:"value"`
	Note  string `json:"note,omitempty"` // e.g. "2023" for a population
}

type Photo struct {
	Thumb   string `json:"thumb"`
	Full    string `json:"full"`
	Page    string `json:"page"`
	Title   string `json:"title,omitempty"`
	Credit  string `json:"credit,omitempty"`
	License string `json:"license,omitempty"`
}

type Link struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

type nominatimPlace struct {
	OSMType     string            `json:"osm_type"`
	OSMID       int64             `json:"osm_id"`
	Lat         string            `json:"lat"`
	Lon         string            `json:"lon"`
	Category    string            `json:"category"`
	Type        string            `json:"type"`
	AddressType string            `json:"addresstype"`
	Name        string            `json:"name"`
	DisplayName string            `json:"display_name"`
	Address     map[string]string `json:"address"`
	ExtraTags   map[string]string `json:"extratags"`
	BoundingBox []string          `json:"boundingbox"` // south, north, west, east
	Error       string            `json:"error"`
}

var osmRefRe = regexp.MustCompile(`^[NWR][0-9]{1,12}$`)

// lookup reads one OSM object (N123, W123, R123) from Nominatim.
func (s *server) lookup(ctx context.Context, osm string) (*nominatimPlace, error) {
	v := url.Values{"osm_ids": {osm}, "format": {"jsonv2"}, "extratags": {"1"},
		"addressdetails": {"1"}, "accept-language": {"en"}}
	var ps []nominatimPlace
	if err := s.up.getJSON(ctx, "https://nominatim.openstreetmap.org/lookup?"+v.Encode(), &ps); err != nil {
		return nil, err
	}
	if len(ps) == 0 {
		return nil, errNotFound
	}
	return &ps[0], nil
}

// reverse finds what is at a point. zoom picks the level of detail, as in
// Nominatim: 3 country, 5 state, 10 city, 14 suburb, 18 building.
func (s *server) reverse(ctx context.Context, lat, lon float64, zoom int) (*nominatimPlace, error) {
	v := url.Values{"lat": {fmtFloat(lat)}, "lon": {fmtFloat(lon)}, "zoom": {strconv.Itoa(zoom)},
		"format": {"jsonv2"}, "extratags": {"1"}, "addressdetails": {"1"}, "accept-language": {"en"}}
	var p nominatimPlace
	if err := s.up.getJSON(ctx, "https://nominatim.openstreetmap.org/reverse?"+v.Encode(), &p); err != nil {
		return nil, err
	}
	if p.Error != "" || p.OSMType == "" {
		return nil, errNotFound
	}
	return &p, nil
}

// findNamed resolves a label clicked on the map (a name at a point) to an
// OSM object. Search results come back with English names while map labels
// may carry the local one ("Stephansdom" for St. Stephen's Cathedral), so
// the nearest hit with that name (and the map's class) within maxKm wins, else the nearest hit
// within nearKm of the map's class (subclass, e.g. "place_of_worship"),
// else the nearest within nearKm under any name.
func (s *server) findNamed(ctx context.Context, name, subclass string, lat, lon, maxKm, nearKm float64) (string, error) {
	rs, err := s.search(ctx, name, &lat, &lon, 14, 10)
	if err != nil {
		return "", err
	}
	named, nameD := "", math.Inf(1)
	near, nearD := "", math.Inf(1)
	same, sameD := "", math.Inf(1)
	namedSame, namedSameD := "", math.Inf(1)
	for _, r := range rs {
		if r.OSM == "" {
			continue
		}
		d := distanceKm(lat, lon, r.Lat, r.Lon)
		if strings.EqualFold(r.Name, name) && d <= maxKm && d < nameD {
			named, nameD = r.OSM, d
		}
		if d <= nearKm && d < nearD {
			near, nearD = r.OSM, d
		}
		if d <= nearKm && subclass != "" && r.osmValue == subclass && d < sameD {
			same, sameD = r.OSM, d
		}
		if strings.EqualFold(r.Name, name) && d <= maxKm && subclass != "" && r.osmValue == subclass && d < namedSameD {
			namedSame, namedSameD = r.OSM, d
		}
	}
	switch {
	case namedSame != "":
		return namedSame, nil
	case named != "":
		return named, nil
	case same != "":
		return same, nil
	case near != "":
		return near, nil
	}
	return "", errNotFound
}

func distanceKm(lat1, lon1, lat2, lon2 float64) float64 {
	const r = 6371
	rad := math.Pi / 180
	dLat, dLon := (lat2-lat1)*rad, (lon2-lon1)*rad
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * r * math.Asin(math.Sqrt(a))
}

// place builds the details of an OSM object: its OSM data, then facts,
// summary and photos from Wikidata, Wikipedia and Wikimedia Commons.
func (s *server) place(ctx context.Context, np *nominatimPlace) (*Place, error) {
	p := &Place{Name: np.Name, OSM: osmRef(np.OSMType, np.OSMID)}
	p.Lat, _ = strconv.ParseFloat(np.Lat, 64)
	p.Lon, _ = strconv.ParseFloat(np.Lon, 64)
	if len(np.BoundingBox) == 4 {
		var b [4]float64
		for i, x := range np.BoundingBox {
			b[i], _ = strconv.ParseFloat(x, 64)
		}
		if b[1]-b[0] > 1e-4 || b[3]-b[2] > 1e-4 {
			p.BBox = []float64{b[2], b[0], b[3], b[1]}
		}
	}
	kind := np.Type
	if np.Category == "boundary" || np.Category == "place" {
		kind = np.AddressType
	}
	p.Kind = kindLabel(np.Category, kind, kind)
	if p.Name == "" {
		p.Name = strings.SplitN(np.DisplayName, ",", 2)[0]
	}
	p.Context, p.Address = addressLines(np, p.Name)
	p.Details = osmDetails(np.ExtraTags)

	qid := np.ExtraTags["wikidata"]
	var wd *wdEntity
	if qidRe.MatchString(qid) {
		var err error
		if wd, err = s.entity(ctx, qid); err != nil && !errors.Is(err, errNotFound) {
			return nil, err
		}
	}

	var (
		wg      sync.WaitGroup
		summary *wpSummary
		nearby  []Photo
		catPics []Photo
	)
	if wd != nil && (np.Category == "boundary" || np.Category == "place") {
		// an area's centroid can lie in the mountains: Wikidata's
		// coordinate is the town itself
		if lat, lon, ok := wd.coord(); ok && (p.BBox == nil || inBBox(p.BBox, lat, lon)) {
			p.Lat, p.Lon = lat, lon
		}
	}
	enTitle := ""
	if wd != nil {
		enTitle = wd.Sitelinks["enwiki"].Title
		p.Description = capitalize(wd.Descriptions["en"].Value)
		if l := wd.Labels["en"].Value; l != "" && np.Name == "" {
			p.Name = l
		}
	}
	if enTitle == "" {
		if t, ok := strings.CutPrefix(np.ExtraTags["wikipedia"], "en:"); ok {
			enTitle = t
		}
	}
	if enTitle != "" {
		wg.Go(func() { summary, _ = s.summary(ctx, enTitle) })
	}
	// Commons category: Wikidata's, else the one tagged in OSM
	osmCommons := np.ExtraTags["wikimedia_commons"]
	cat := ""
	if wd != nil {
		cat = wd.str("P373")
	}
	if c, ok := strings.CutPrefix(osmCommons, "Category:"); ok && cat == "" {
		cat = c
	}
	if cat != "" {
		wg.Go(func() { catPics, _ = s.categoryPhotos(ctx, cat, 10) })
	}
	var sitePic *Photo
	if site := firstOf(np.ExtraTags, "website", "contact:website", "url"); site != "" {
		wg.Go(func() { sitePic = s.previewImage(ctx, site, p.Name) })
	}
	// An area gets what was photographed around it. A point of interest
	// only gets nearby photos that name it: around a remote lodge, the
	// nearest Commons photo can be a moth specimen.
	area := false
	radius := 300
	if p.BBox != nil {
		// a radius that fits the place: a few km for a town, capped at 10 km
		w := distanceKm(p.BBox[1], p.BBox[0], p.BBox[1], p.BBox[2])
		radius = int(math.Min(10000, math.Max(300, w*1000/4)))
		area = w >= 1
	}
	wg.Go(func() { nearby, _ = s.nearbyPhotos(ctx, p.Lat, p.Lon, radius, 12) })
	var facts []Fact
	if wd != nil {
		wg.Go(func() { facts = s.facts(ctx, wd) })
	}
	wg.Wait()
	p.Facts = facts
	p.Details = dropRepeats(p.Details, facts)

	// Photos: the Wikidata image, the one tagged in OSM, the website's
	// preview image, the Commons category, then what was taken nearby.
	seen := map[string]bool{}
	add := func(ph Photo) {
		if ph.Thumb != "" && !seen[ph.Page] && len(p.Photos) < 16 {
			seen[ph.Page] = true
			p.Photos = append(p.Photos, ph)
		}
	}
	var files []string
	if wd != nil {
		files = append(files, wd.str("P18"))
	}
	if f, ok := strings.CutPrefix(osmCommons, "File:"); ok {
		files = append(files, f)
	}
	for _, f := range files {
		if f == "" {
			continue
		}
		if ph, err := s.filePhoto(ctx, f); err == nil {
			add(ph)
		}
	}
	if sitePic != nil {
		add(*sitePic)
	}
	for _, ph := range catPics {
		add(ph)
	}
	for _, ph := range nearby {
		if area || namesPlace(ph.Title, p.Name) {
			add(ph)
		}
	}

	if summary != nil {
		p.Summary = summary.Extract
		if p.Description == "" {
			p.Description = capitalize(summary.Description)
		}
	}
	p.Links = links(p, np, qid, enTitle)
	return p, nil
}

// namesPlace reports whether a photo title shares a word of four or more
// letters with the place's name.
func namesPlace(title, name string) bool {
	t := strings.ToLower(title)
	for _, w := range strings.FieldsFunc(strings.ToLower(name), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		if len([]rune(w)) >= 4 && strings.Contains(t, w) {
			return true
		}
	}
	return false
}

func inBBox(b []float64, lat, lon float64) bool {
	return lon >= b[0] && lat >= b[1] && lon <= b[2] && lat <= b[3]
}

func osmRef(typ string, id int64) string {
	if typ == "" || id == 0 {
		return ""
	}
	return strings.ToUpper(typ[:1]) + strconv.FormatInt(id, 10)
}

// addressLines gives the region a place lies in ("Upper Austria, Austria")
// and, for things with a street address, that address.
func addressLines(np *nominatimPlace, name string) (context, address string) {
	a := np.Address
	city := firstOf(a, "city", "town", "village", "hamlet", "municipality")
	context = joinNonEmpty(", ", name, city, firstOf(a, "county", "state_district"), a["state"], a["country"])
	if road := a["road"]; road != "" {
		address = joinNonEmpty(", ", "", strings.TrimSpace(road+" "+a["house_number"]),
			strings.TrimSpace(a["postcode"]+" "+city))
	}
	return
}

func firstOf(m map[string]string, keys ...string) string {
	for _, k := range keys {
		if m[k] != "" {
			return m[k]
		}
	}
	return ""
}

// osmDetails picks the OSM tags worth showing, under English labels.
func osmDetails(tags map[string]string) []Fact {
	pick := []struct{ label, keys string }{
		{"Opening hours", "opening_hours"},
		{"Cuisine", "cuisine"},
		{"Phone", "phone contact:phone"},
		{"Website", "website contact:website url"},
		{"Email", "email contact:email"},
		{"Wheelchair", "wheelchair"},
		{"Operator", "operator"},
		{"Brand", "brand"},
		{"Takeaway", "takeaway"},
		{"Delivery", "delivery"},
		{"Outdoor seating", "outdoor_seating"},
		{"Stars", "stars"},
		{"Fee", "fee"},
		{"Denomination", "denomination"},
	}
	var out []Fact
	for _, p := range pick {
		for _, k := range strings.Fields(p.keys) {
			if v := tags[k]; v != "" {
				if k == "cuisine" || k == "denomination" {
					v = capitalize(strings.ReplaceAll(strings.ReplaceAll(v, "_", " "), ";", ", "))
				}
				out = append(out, Fact{Label: p.label, Value: v})
				break
			}
		}
	}
	return out
}

// dropRepeats removes details that the facts already show.
func dropRepeats(details, facts []Fact) []Fact {
	shown := map[string]bool{}
	for _, f := range facts {
		shown[f.Label+"\x00"+f.Value] = true
	}
	var out []Fact
	for _, d := range details {
		if !shown[d.Label+"\x00"+d.Value] {
			out = append(out, d)
		}
	}
	return out
}

func links(p *Place, np *nominatimPlace, qid, enTitle string) []Link {
	ll := fmt.Sprintf("%.6f,%.6f", p.Lat, p.Lon)
	q := url.QueryEscape(joinNonEmpty(", ", "", p.Name, p.Context))
	out := []Link{
		{"Google Maps", "https://www.google.com/maps/search/?api=1&query=" + q},
		{"Apple Maps", "https://maps.apple.com/?q=" + url.QueryEscape(p.Name) + "&ll=" + ll},
	}
	if np.OSMType != "" {
		out = append(out, Link{"OpenStreetMap", fmt.Sprintf("https://www.openstreetmap.org/%s/%d", np.OSMType, np.OSMID)})
	}
	if enTitle != "" {
		out = append(out, Link{"Wikipedia", "https://en.wikipedia.org/wiki/" + url.PathEscape(strings.ReplaceAll(enTitle, " ", "_"))})
	}
	if qid != "" {
		out = append(out, Link{"Wikidata", "https://www.wikidata.org/wiki/" + qid})
	}
	return out
}
