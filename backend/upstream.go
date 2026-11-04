package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// userAgent identifies terra to the public APIs it uses (Nominatim and
// Wikimedia require a descriptive one).
const userAgent = "terra/1.0 (+https://github.com/dseif0x/terra)"

const (
	cacheTTL      = 24 * time.Hour
	cacheMaxBytes = 64 << 20
	maxBody       = 32 << 20
)

// upstream fetches JSON from the public APIs with a shared response cache
// and a minimum interval between requests per host (Nominatim's usage
// policy allows one request per second).
type upstream struct {
	client   *http.Client
	interval map[string]time.Duration

	mu    sync.Mutex
	next  map[string]time.Time
	cache map[string]cacheEntry
	order []string // insertion order, for eviction
	bytes int
}

type cacheEntry struct {
	body    []byte
	expires time.Time
}

func newUpstream() *upstream {
	return &upstream{
		client: &http.Client{Timeout: 20 * time.Second},
		interval: map[string]time.Duration{
			"nominatim.openstreetmap.org": time.Second,
			"photon.komoot.io":            200 * time.Millisecond,
		},
		next:  map[string]time.Time{},
		cache: map[string]cacheEntry{},
	}
}

var errNotFound = errors.New("not found")

// getJSON fetches u (from the cache when possible) and decodes it into out.
func (up *upstream) getJSON(ctx context.Context, u string, out any) error {
	body, err := up.get(ctx, u)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("%s: %w", hostOf(u), err)
	}
	return nil
}

func (up *upstream) get(ctx context.Context, u string) ([]byte, error) {
	if b, ok := up.cached(u); ok {
		return b, nil
	}
	host := hostOf(u)
	if err := up.wait(ctx, host); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "en")
	resp, err := up.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, errNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", host, resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxBody {
		return nil, fmt.Errorf("%s: response too large", host)
	}
	up.store(u, body)
	return body, nil
}

// wait blocks until a request to host may go out.
func (up *upstream) wait(ctx context.Context, host string) error {
	gap := up.interval[host]
	if gap == 0 {
		return nil
	}
	up.mu.Lock()
	now := time.Now()
	at := up.next[host]
	if at.Before(now) {
		at = now
	}
	up.next[host] = at.Add(gap)
	up.mu.Unlock()
	if d := time.Until(at); d > 0 {
		t := time.NewTimer(d)
		defer t.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
		}
	}
	return nil
}

func (up *upstream) cached(u string) ([]byte, bool) {
	up.mu.Lock()
	defer up.mu.Unlock()
	e, ok := up.cache[u]
	if !ok || time.Now().After(e.expires) {
		return nil, false
	}
	return e.body, true
}

func (up *upstream) store(u string, body []byte) {
	if len(body) > cacheMaxBytes/8 {
		return
	}
	up.mu.Lock()
	defer up.mu.Unlock()
	if old, ok := up.cache[u]; ok {
		up.bytes -= len(old.body)
	} else {
		up.order = append(up.order, u)
	}
	up.cache[u] = cacheEntry{body: body, expires: time.Now().Add(cacheTTL)}
	up.bytes += len(body)
	for up.bytes > cacheMaxBytes && len(up.order) > 0 {
		k := up.order[0]
		up.order = up.order[1:]
		if e, ok := up.cache[k]; ok {
			up.bytes -= len(e.body)
			delete(up.cache, k)
		}
	}
}

func hostOf(u string) string {
	p, err := url.Parse(u)
	if err != nil {
		return ""
	}
	return p.Hostname()
}
