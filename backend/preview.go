package main

import (
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"syscall"
	"time"
)

// previewImage reads the og:image (or twitter:image) of a place's website,
// the photo a business picks for itself. The website comes from OSM, so
// anyone can set it: the client refuses private and loopback addresses
// (no requests into the cluster) and reads at most 512 KB.
func (s *server) previewImage(ctx context.Context, site, name string) *Photo {
	u, err := url.Parse(site)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return nil
	}
	key := "preview:" + u.String()
	if b, ok := s.up.cached(key); ok {
		return previewPhoto(string(b), u, name)
	}
	img, err := fetchPreview(ctx, u)
	if err != nil {
		img = "" // remember failures too, for the cache's lifetime
	}
	s.up.store(key, []byte(img))
	return previewPhoto(img, u, name)
}

func previewPhoto(img string, site *url.URL, name string) *Photo {
	if img == "" {
		return nil
	}
	return &Photo{Thumb: img, Full: img, Page: site.String(), Title: name,
		Credit: strings.TrimPrefix(site.Hostname(), "www."), License: "from the website"}
}

var (
	metaRe    = regexp.MustCompile(`(?is)<meta\s[^>]*>`)
	propRe    = regexp.MustCompile(`(?is)(?:property|name)\s*=\s*["']?(og:image(?::secure_url|:url)?|twitter:image(?::src)?)["'\s/>]`)
	contentRe = regexp.MustCompile(`(?is)content\s*=\s*(?:"([^"]*)"|'([^']*)')`)
)

// ogImage finds the preview image in a page's head.
func ogImage(page string, base *url.URL) string {
	for _, m := range metaRe.FindAllString(page, -1) {
		if !propRe.MatchString(m) {
			continue
		}
		c := contentRe.FindStringSubmatch(m)
		if c == nil {
			continue
		}
		v := strings.TrimSpace(html.UnescapeString(c[1] + c[2]))
		ref, err := url.Parse(v)
		if err != nil || v == "" {
			continue
		}
		abs := base.ResolveReference(ref)
		if abs.Scheme == "http" {
			abs.Scheme = "https" // the page is https: plain http images are blocked
		}
		if abs.Scheme == "https" {
			return abs.String()
		}
	}
	return ""
}

var previewClient = &http.Client{
	Timeout: 6 * time.Second,
	Transport: &http.Transport{
		Proxy: nil,
		DialContext: (&net.Dialer{
			Timeout: 4 * time.Second,
			Control: func(network, address string, _ syscall.RawConn) error {
				host, _, err := net.SplitHostPort(address)
				if err != nil {
					return err
				}
				ip := net.ParseIP(host)
				if ip == nil || !publicIP(ip) {
					return fmt.Errorf("refusing to connect to %s", host)
				}
				return nil
			},
		}).DialContext,
		TLSHandshakeTimeout:   4 * time.Second,
		ResponseHeaderTimeout: 5 * time.Second,
		MaxIdleConns:          10,
		IdleConnTimeout:       30 * time.Second,
	},
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("too many redirects")
		}
		if req.URL.Scheme != "https" && req.URL.Scheme != "http" {
			return errors.New("bad redirect")
		}
		return nil
	},
}

func publicIP(ip net.IP) bool {
	return ip.IsGlobalUnicast() && !ip.IsPrivate() && !ip.IsLoopback() &&
		!ip.IsLinkLocalUnicast() && !cgnat.Contains(ip)
}

var cgnat = &net.IPNet{IP: net.IPv4(100, 64, 0, 0), Mask: net.CIDRMask(10, 32)}

func fetchPreview(ctx context.Context, u *url.URL) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html")
	resp, err := previewClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.Contains(resp.Header.Get("Content-Type"), "html") {
		return "", fmt.Errorf("preview: %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 512<<10))
	if err != nil {
		return "", err
	}
	return ogImage(string(body), resp.Request.URL), nil
}
