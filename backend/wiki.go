package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"math"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var qidRe = regexp.MustCompile(`^Q[0-9]{1,12}$`)

type wdText struct {
	Value string `json:"value"`
}

type wdSnak struct {
	SnakType  string `json:"snaktype"`
	DataValue struct {
		Type  string          `json:"type"`
		Value json.RawMessage `json:"value"`
	} `json:"datavalue"`
}

type wdClaim struct {
	MainSnak   wdSnak              `json:"mainsnak"`
	Rank       string              `json:"rank"`
	Qualifiers map[string][]wdSnak `json:"qualifiers"`
}

type wdEntity struct {
	ID           string               `json:"id"`
	Labels       map[string]wdText    `json:"labels"`
	Descriptions map[string]wdText    `json:"descriptions"`
	Claims       map[string][]wdClaim `json:"claims"`
	Sitelinks    map[string]struct {
		Title string `json:"title"`
	} `json:"sitelinks"`
}

type wdQuantity struct {
	Amount string `json:"amount"`
	Unit   string `json:"unit"`
}

type wdTime struct {
	Time      string `json:"time"`
	Precision int    `json:"precision"`
}

func (s *server) entity(ctx context.Context, qid string) (*wdEntity, error) {
	v := url.Values{"action": {"wbgetentities"}, "ids": {qid}, "format": {"json"},
		"props": {"labels|descriptions|claims|sitelinks"}, "languages": {"en"}, "sitefilter": {"enwiki"}}
	var r struct {
		Entities map[string]wdEntity `json:"entities"`
	}
	if err := s.up.getJSON(ctx, "https://www.wikidata.org/w/api.php?"+v.Encode(), &r); err != nil {
		return nil, err
	}
	e, ok := r.Entities[qid]
	if !ok || e.Claims == nil && e.Labels == nil {
		return nil, errNotFound
	}
	return &e, nil
}

// labels resolves entity ids to their English labels.
func (s *server) labels(ctx context.Context, ids []string) map[string]string {
	out := map[string]string{}
	for len(ids) > 0 {
		n := min(len(ids), 50)
		v := url.Values{"action": {"wbgetentities"}, "ids": {strings.Join(ids[:n], "|")},
			"props": {"labels"}, "languages": {"en"}, "format": {"json"}}
		ids = ids[n:]
		var r struct {
			Entities map[string]wdEntity `json:"entities"`
		}
		if err := s.up.getJSON(ctx, "https://www.wikidata.org/w/api.php?"+v.Encode(), &r); err != nil {
			continue
		}
		for id, e := range r.Entities {
			if l := e.Labels["en"].Value; l != "" {
				out[id] = l
			}
		}
	}
	return out
}

// best returns the claims of a property worth showing: the preferred ones
// if any, else the normal ones (deprecated ones never).
func (e *wdEntity) best(prop string) []wdClaim {
	var pref, norm []wdClaim
	for _, c := range e.Claims[prop] {
		if c.MainSnak.SnakType != "value" {
			continue
		}
		switch c.Rank {
		case "preferred":
			pref = append(pref, c)
		case "normal":
			norm = append(norm, c)
		}
	}
	if len(pref) > 0 {
		return pref
	}
	return norm
}

// str is the first string value of a property (file names, categories, URLs).
func (e *wdEntity) str(prop string) string {
	for _, c := range e.best(prop) {
		var v string
		if json.Unmarshal(c.MainSnak.DataValue.Value, &v) == nil && v != "" {
			return v
		}
	}
	return ""
}

// coord is the entity's coordinate location (P625).
func (e *wdEntity) coord() (lat, lon float64, ok bool) {
	for _, c := range e.best("P625") {
		var v struct {
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
			Globe     string  `json:"globe"`
		}
		if json.Unmarshal(c.MainSnak.DataValue.Value, &v) == nil && strings.HasSuffix(v.Globe, "/Q2") {
			return v.Latitude, v.Longitude, true
		}
	}
	return 0, 0, false
}

func snakID(s wdSnak) string {
	var v struct {
		ID string `json:"id"`
	}
	json.Unmarshal(s.DataValue.Value, &v)
	return v.ID
}

func snakQuantity(s wdSnak) (float64, string, bool) {
	var q wdQuantity
	if json.Unmarshal(s.DataValue.Value, &q) != nil {
		return 0, "", false
	}
	f, err := strconv.ParseFloat(strings.TrimPrefix(q.Amount, "+"), 64)
	if err != nil {
		return 0, "", false
	}
	return f, strings.TrimPrefix(q.Unit, "http://www.wikidata.org/entity/"), true
}

func snakTime(s wdSnak) (wdTime, bool) {
	var t wdTime
	if json.Unmarshal(s.DataValue.Value, &t) != nil || t.Time == "" {
		return t, false
	}
	return t, true
}

// year returns the (possibly negative) year of a Wikidata time.
func (t wdTime) year() int {
	s := t.Time
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimLeft(s, "+-")
	y, _ := strconv.Atoi(strings.SplitN(s, "-", 2)[0])
	if neg {
		return -y
	}
	return y
}

func (t wdTime) String() string {
	y := t.year()
	switch {
	case t.Precision <= 6: // millennium or coarser
		return yearString(y)
	case t.Precision == 7:
		c := (abs(y)-1)/100 + 1
		s := ordinal(c) + " century"
		if y < 0 {
			s += " BC"
		}
		return s
	case t.Precision == 8:
		return fmt.Sprintf("%ds", y/10*10)
	}
	return yearString(y)
}

func yearString(y int) string {
	if y < 0 {
		return strconv.Itoa(-y) + " BC"
	}
	return strconv.Itoa(y)
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func ordinal(n int) string {
	suffix := "th"
	if n%100 < 11 || n%100 > 13 {
		switch n % 10 {
		case 1:
			suffix = "st"
		case 2:
			suffix = "nd"
		case 3:
			suffix = "rd"
		}
	}
	return strconv.Itoa(n) + suffix
}

// qualifierTime reads a time qualifier (P585 point in time, P580 start).
func qualifierTime(c wdClaim, prop string) (wdTime, bool) {
	for _, q := range c.Qualifiers[prop] {
		if t, ok := snakTime(q); ok {
			return t, true
		}
	}
	return wdTime{}, false
}

// latest picks the claim with the latest point in time (or start time).
func latest(cs []wdClaim, prop string) (wdClaim, bool) {
	if len(cs) == 0 {
		return wdClaim{}, false
	}
	best, by := cs[0], math.MinInt
	for _, c := range cs {
		if t, ok := qualifierTime(c, prop); ok && t.year() > by {
			best, by = c, t.year()
		}
	}
	return best, true
}

// current drops claims that have ended (P582 end time).
func current(cs []wdClaim) []wdClaim {
	var out []wdClaim
	for _, c := range cs {
		if len(c.Qualifiers["P582"]) == 0 {
			out = append(out, c)
		}
	}
	return out
}

// Units of length and area that Wikidata uses most.
var (
	areaKm2 = map[string]float64{"Q712226": 1, "Q25343": 1e-6, "Q35852": 0.01, "Q232291": 2.589988, "Q81292": 0.004046856}
	lengthM = map[string]float64{"Q11573": 1, "Q3710": 0.3048, "Q828224": 1000}
)

// facts turns an entity's claims into the facts panel.
func (s *server) facts(ctx context.Context, e *wdEntity) []Fact {
	type pending struct {
		label string
		ids   []string
	}
	var out []Fact
	var refs []pending  // facts whose values are entities, resolved below
	at := map[int]int{} // refs index -> out index
	entityFact := func(label, prop string, max int, onlyCurrent bool) {
		cs := e.best(prop)
		if onlyCurrent {
			cs = current(cs)
		}
		var ids []string
		for _, c := range cs {
			if id := snakID(c.MainSnak); id != "" && len(ids) < max {
				ids = append(ids, id)
			}
		}
		if len(ids) > 0 {
			at[len(refs)] = len(out)
			refs = append(refs, pending{label, ids})
			out = append(out, Fact{Label: label})
		}
	}

	var popVal, areaVal float64
	if c, ok := latest(e.best("P1082"), "P585"); ok {
		if v, _, ok := snakQuantity(c.MainSnak); ok && v > 0 {
			popVal = v
			f := Fact{Label: "Population", Value: formatInt(v)}
			if t, ok := qualifierTime(c, "P585"); ok {
				f.Note = yearString(t.year())
			}
			out = append(out, f)
		}
	}
	if c, ok := latest(e.best("P2046"), "P585"); ok {
		if v, unit, ok := snakQuantity(c.MainSnak); ok && areaKm2[unit] > 0 && v > 0 {
			areaVal = v * areaKm2[unit]
			out = append(out, Fact{Label: "Area", Value: formatArea(areaVal)})
		}
	}
	if popVal > 0 && areaVal > 0 {
		out = append(out, Fact{Label: "Density", Value: formatInt(popVal/areaVal) + " / km²"})
	}
	for _, p := range []struct{ label, prop string }{{"Elevation", "P2044"}, {"Height", "P2048"}} {
		if cs := e.best(p.prop); len(cs) > 0 {
			if v, unit, ok := snakQuantity(cs[0].MainSnak); ok && lengthM[unit] > 0 {
				out = append(out, Fact{Label: p.label, Value: formatInt(v*lengthM[unit]) + " m"})
			}
		}
	}
	entityFact("Capital", "P36", 1, true)
	entityFact("Country", "P17", 1, true)
	entityFact("Located in", "P131", 1, true)
	for _, p := range []struct{ label, prop string }{{"Founded", "P571"}, {"Opened", "P1619"}} {
		if cs := e.best(p.prop); len(cs) > 0 {
			if t, ok := snakTime(cs[0].MainSnak); ok {
				out = append(out, Fact{Label: p.label, Value: t.String()})
			}
		}
	}
	entityFact("Languages", "P37", 3, true)
	entityFact("Currency", "P38", 1, true)
	entityFact("Time zone", "P421", 1, true)
	entityFact("Head of state", "P35", 1, true)
	entityFact("Head of government", "P6", 1, true)
	entityFact("Architect", "P84", 2, false)
	entityFact("Heritage", "P1435", 1, false)

	if len(refs) > 0 {
		var ids []string
		for _, r := range refs {
			ids = append(ids, r.ids...)
		}
		names := s.labels(ctx, dedupe(ids))
		for i, r := range refs {
			var vs []string
			for _, id := range r.ids {
				if n := names[id]; n != "" {
					vs = append(vs, n)
				}
			}
			out[at[i]].Value = strings.Join(vs, ", ")
		}
	}
	if site := e.str("P856"); site != "" {
		out = append(out, Fact{Label: "Website", Value: site})
	}
	// drop entity facts whose labels didn't resolve, and facts that repeat
	// the place itself ("Located in: Austria" next to "Country: Austria")
	var clean []Fact
	seen := map[string]bool{}
	for _, f := range out {
		if f.Value == "" || (f.Label != "Population" && seen[f.Value]) {
			continue
		}
		seen[f.Value] = true
		clean = append(clean, f)
	}
	return clean
}

func dedupe(xs []string) []string {
	sort.Strings(xs)
	var out []string
	for i, x := range xs {
		if i == 0 || x != xs[i-1] {
			out = append(out, x)
		}
	}
	return out
}

// formatInt rounds and groups thousands: 1234567 -> "1,234,567".
func formatInt(f float64) string {
	n := int64(math.Round(f))
	neg := n < 0
	if neg {
		n = -n
	}
	s := strconv.FormatInt(n, 10)
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	if neg {
		return "-" + b.String()
	}
	return b.String()
}

func formatArea(km2 float64) string {
	switch {
	case km2 >= 100:
		return formatInt(km2) + " km²"
	case km2 >= 1:
		return strconv.FormatFloat(math.Round(km2*10)/10, 'f', -1, 64) + " km²"
	case km2 >= 0.01:
		return formatInt(km2*100) + " ha"
	}
	return formatInt(km2*1e6) + " m²"
}

type wpSummary struct {
	Type        string `json:"type"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Extract     string `json:"extract"`
}

func (s *server) summary(ctx context.Context, title string) (*wpSummary, error) {
	var r wpSummary
	u := "https://en.wikipedia.org/api/rest_v1/page/summary/" + url.PathEscape(strings.ReplaceAll(title, " ", "_"))
	if err := s.up.getJSON(ctx, u, &r); err != nil {
		return nil, err
	}
	if r.Type == "disambiguation" {
		return nil, errNotFound
	}
	return &r, nil
}

// Commons photos ---------------------------------------------------------

const (
	thumbWidth = 500
	fullWidth  = 1920
)

type commonsQuery struct {
	Query struct {
		Pages map[string]struct {
			Title     string `json:"title"`
			Index     int    `json:"index"`
			ImageInfo []struct {
				URL         string `json:"url"`
				ThumbURL    string `json:"thumburl"`
				DescURL     string `json:"descriptionurl"`
				Width       int    `json:"width"`
				Mime        string `json:"mime"`
				ExtMetadata map[string]struct {
					Value any `json:"value"`
				} `json:"extmetadata"`
			} `json:"imageinfo"`
		} `json:"pages"`
	} `json:"query"`
}

func (s *server) commons(ctx context.Context, v url.Values) ([]Photo, error) {
	v.Set("action", "query")
	v.Set("format", "json")
	v.Set("prop", "imageinfo")
	v.Set("iiprop", "url|size|mime|extmetadata")
	v.Set("iiurlwidth", strconv.Itoa(thumbWidth))
	v.Set("iiextmetadatafilter", "Artist|LicenseShortName|ObjectName")
	var r commonsQuery
	if err := s.up.getJSON(ctx, "https://commons.wikimedia.org/w/api.php?"+v.Encode(), &r); err != nil {
		return nil, err
	}
	type ranked struct {
		i int
		p Photo
	}
	var rs []ranked
	for _, pg := range r.Query.Pages {
		if len(pg.ImageInfo) == 0 {
			continue
		}
		ii := pg.ImageInfo[0]
		if ii.Mime != "image/jpeg" && ii.Mime != "image/webp" {
			continue // svg maps, coats of arms, diagrams, videos
		}
		full := ii.URL
		if ii.Width > fullWidth && ii.ThumbURL != "" {
			full = strings.Replace(ii.ThumbURL, fmt.Sprintf("/%dpx-", thumbWidth), fmt.Sprintf("/%dpx-", fullWidth), 1)
		}
		meta := func(k string) string {
			if m, ok := ii.ExtMetadata[k]; ok {
				if s, ok := m.Value.(string); ok {
					return stripTags(s)
				}
			}
			return ""
		}
		title := meta("ObjectName")
		if title == "" {
			title = strings.TrimPrefix(pg.Title, "File:")
			if i := strings.LastIndex(title, "."); i > 0 {
				title = title[:i]
			}
		}
		rs = append(rs, ranked{pg.Index, Photo{
			Thumb: ii.ThumbURL, Full: full, Page: ii.DescURL, Title: title,
			Credit: meta("Artist"), License: meta("LicenseShortName"),
		}})
	}
	sort.Slice(rs, func(i, j int) bool { return rs[i].i < rs[j].i })
	out := make([]Photo, len(rs))
	for i, r := range rs {
		out[i] = r.p
	}
	return out, nil
}

func (s *server) filePhoto(ctx context.Context, file string) (Photo, error) {
	ps, err := s.commons(ctx, url.Values{"titles": {"File:" + file}})
	if err != nil {
		return Photo{}, err
	}
	if len(ps) == 0 {
		return Photo{}, errNotFound
	}
	return ps[0], nil
}

func (s *server) categoryPhotos(ctx context.Context, cat string, n int) ([]Photo, error) {
	return s.commons(ctx, url.Values{"generator": {"categorymembers"}, "gcmtitle": {"Category:" + cat},
		"gcmtype": {"file"}, "gcmlimit": {strconv.Itoa(n * 2)}})
}

func (s *server) nearbyPhotos(ctx context.Context, lat, lon float64, radius, n int) ([]Photo, error) {
	return s.commons(ctx, url.Values{"generator": {"geosearch"}, "ggsnamespace": {"6"},
		"ggscoord": {fmtFloat(lat) + "|" + fmtFloat(lon)}, "ggsradius": {strconv.Itoa(radius)},
		"ggslimit": {strconv.Itoa(n * 2)}})
}

var tagRe = regexp.MustCompile(`<[^>]*>`)

func stripTags(s string) string {
	s = html.UnescapeString(tagRe.ReplaceAllString(s, ""))
	return strings.Join(strings.Fields(s), " ")
}
