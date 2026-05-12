package fronter

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/andybalholm/brotli"
	"mhrv-go/config"
)

// ── Rate Limiter ──────────────────────────────────────────────────────────

type rateLimiter struct {
	mu       sync.Mutex
	buckets  map[string]*bucket
	rate     int
	interval time.Duration
}

type bucket struct {
	tokens float64
	last   time.Time
}

func newRateLimiter(rate int, interval time.Duration) *rateLimiter {
	return &rateLimiter{
		buckets:  make(map[string]*bucket),
		rate:     rate,
		interval: interval,
	}
}

func (rl *rateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	b, ok := rl.buckets[key]
	now := time.Now()
	if !ok {
		b = &bucket{tokens: float64(rl.rate), last: now}
		rl.buckets[key] = b
	}
	elapsed := now.Sub(b.last).Seconds()
	b.tokens += elapsed * float64(rl.rate) / rl.interval.Seconds()
	if b.tokens > float64(rl.rate) {
		b.tokens = float64(rl.rate)
	}
	b.last = now
	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

// ── Coalescing Group ──────────────────────────────────────────────────────

type coalesceGroup struct {
	sync.Mutex
	done   bool
	result *http.Response
	err    error
}

// ── AppsScriptRelay ───────────────────────────────────────────────────────

type AppsScriptRelay struct {
	cfg        config.Config
	httpClient *http.Client

	devAvailable bool
	devMu        sync.Mutex

	blacklistMu sync.Mutex
	blacklist   map[string]time.Time

	coalesceMu  sync.Mutex
	coalesceMap map[string]*coalesceGroup

	hostLimiter *rateLimiter
}

func NewRelay(cfg config.Config) *AppsScriptRelay {
	tr := &http.Transport{
		DialTLSContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			d := net.Dialer{Timeout: 12 * time.Second}
			conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(cfg.GoogleIP, "443"))
			if err != nil {
				return nil, err
			}
			tlsCfg := &tls.Config{ServerName: cfg.FrontDomain}
			if !cfg.VerifySSL {
				tlsCfg.InsecureSkipVerify = true
			}
			tlsConn := tls.Client(conn, tlsCfg)
			if err := tlsConn.Handshake(); err != nil {
				conn.Close()
				return nil, err
			}
			return tlsConn, nil
		},
		MaxIdleConns:       10,
		IdleConnTimeout:    30 * time.Second,
		DisableCompression: false,
		ForceAttemptHTTP2:  false,
	}
	return &AppsScriptRelay{
		cfg: cfg,
		httpClient: &http.Client{
			Transport: tr,
			Timeout:   25 * time.Second,
		},
		blacklist:   make(map[string]time.Time),
		coalesceMap: make(map[string]*coalesceGroup),
		hostLimiter: newRateLimiter(100, 200*time.Millisecond),
	}
}

// ── Script ID management ─────────────────────────────────────────────────

func (r *AppsScriptRelay) collectScriptIDs() []string {
	var ids []string
	if r.cfg.ScriptID != "" {
		ids = append(ids, r.cfg.ScriptID)
	}
	ids = append(ids, r.cfg.ScriptIDs...)
	for _, acc := range r.cfg.Accounts {
		ids = append(ids, acc.ScriptIDs...)
	}
	return ids
}

func (r *AppsScriptRelay) getScriptIDs() []string {
	ids := r.collectScriptIDs()
	if len(ids) == 0 {
		return ids
	}
	r.blacklistMu.Lock()
	defer r.blacklistMu.Unlock()
	now := time.Now()
	healthy := make([]string, 0, len(ids))
	for _, id := range ids {
		if until, ok := r.blacklist[id]; ok && now.Before(until) {
			continue
		}
		healthy = append(healthy, id)
	}
	if len(healthy) == 0 {
		// all blacklisted – clear expired and use all
		for id, until := range r.blacklist {
			if now.After(until) {
				delete(r.blacklist, id)
			}
		}
		healthy = ids
	}
	return healthy
}

func (r *AppsScriptRelay) blacklistScript(id string) {
	r.blacklistMu.Lock()
	r.blacklist[id] = time.Now().Add(10 * time.Minute)
	r.blacklistMu.Unlock()
	log.Printf("Blacklisted script %s for 10 minutes", id[:12]+"...")
}

// ── Permanent error detection (narrowed) ─────────────────────────────────

func isPermanentError(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	// Only blacklist for truly permanent or quota-related errors.
	return strings.Contains(s, "relay error") || // covers quota, unauthorized, etc.
		strings.Contains(s, "unauthorized") ||
		strings.Contains(s, "loop detected") ||
		strings.Contains(s, "سرویس در طول یک روز")
}

// ── Main entry point ─────────────────────────────────────────────────────

func (r *AppsScriptRelay) Do(method, targetURL string, headers map[string]string, body []byte) (*http.Response, error) {
	host := extractHost(targetURL)
	if !r.hostLimiter.allow(host) {
		return nil, fmt.Errorf("rate limit exceeded for %s", host)
	}

	// Exit Node logic (if enabled in GUI)
	if r.cfg.ExitNode.Enabled && r.cfg.ExitNode.RelayURL != "" && r.shouldUseExitNode(host) {
		log.Printf("Exit Node: routing %s via %s", host, r.cfg.ExitNode.RelayURL)
		return r.relayViaExitNode(method, targetURL, headers, body)
	}

	// Coalescing for GET requests
	if method == "GET" && body == nil {
		key := method + "|" + targetURL
		if resp, err := r.coalescedGet(key, targetURL, headers); resp != nil || err != nil {
			return resp, err
		}
	}

	ids := r.getScriptIDs()
	if len(ids) == 0 {
		return nil, fmt.Errorf("no Script ID configured")
	}
	var lastErr error
	for _, sid := range ids {
		resp, err := r.doRequest(sid, method, targetURL, headers, body)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if isPermanentError(err) {
			r.blacklistScript(sid)
		} else {
			log.Printf("Transient error for script %s: %v", sid[:12], err)
		}
	}
	return nil, lastErr
}

func (r *AppsScriptRelay) shouldUseExitNode(host string) bool {
	if !r.cfg.ExitNode.Enabled {
		return false
	}
	mode := r.cfg.ExitNode.Mode
	if mode == "" {
		mode = "selective"
	}
	if mode == "full" {
		return true
	}
	host = strings.TrimRight(host, ".")
	for _, h := range r.cfg.ExitNode.Hosts {
		h = strings.TrimSpace(h)
		h = strings.TrimRight(h, ".")
		if h == "" {
			continue
		}
		if h == host {
			return true
		}
		if strings.HasPrefix(h, ".") && strings.HasSuffix(host, h) {
			return true
		}
	}
	return false
}

// ── Exit Node relay ──────────────────────────────────────────────────────

func (r *AppsScriptRelay) relayViaExitNode(method, targetURL string, headers map[string]string, body []byte) (*http.Response, error) {
	inner := map[string]interface{}{
		"m": method,
		"u": targetURL,
		"r": false,
		"k": r.cfg.ExitNode.PSK,
	}
	if cleanH := cleanReqHeaders(headers); len(cleanH) > 0 {
		inner["h"] = cleanH
	}
	if len(body) > 0 {
		inner["b"] = base64.StdEncoding.EncodeToString(body)
		if ct, ok := headers["Content-Type"]; ok {
			inner["ct"] = ct
		} else if ct, ok := headers["content-type"]; ok {
			inner["ct"] = ct
		}
	}

	innerJSON, _ := json.Marshal(inner)
	outer := map[string]interface{}{
		"m":  "POST",
		"u":  r.cfg.ExitNode.RelayURL,
		"r":  false,
		"k":  r.cfg.AuthKey,
		"ct": "application/json",
		"b":  base64.StdEncoding.EncodeToString(innerJSON),
	}

	ids := r.getScriptIDs()
	for _, sid := range ids {
		execURL := fmt.Sprintf("https://script.google.com/macros/s/%s/exec", sid)
		if r.devAvailable {
			execURL = fmt.Sprintf("https://script.google.com/macros/s/%s/dev", sid)
		}
		resp, err := r.sendSingle(execURL, outer)
		if err == nil {
			return resp, nil
		}
		log.Printf("Exit node attempt failed with script %s: %v", sid[:12], err)
	}
	return nil, fmt.Errorf("all exit node attempts failed")
}

// ── Request handling ─────────────────────────────────────────────────────

func (r *AppsScriptRelay) coalescedGet(key, targetURL string, headers map[string]string) (*http.Response, error) {
	r.coalesceMu.Lock()
	group, exists := r.coalesceMap[key]
	if !exists {
		group = &coalesceGroup{}
		r.coalesceMap[key] = group
		r.coalesceMu.Unlock()

		resp, err := r.doRequestSingle(targetURL, headers)
		group.Lock()
		group.result = resp
		group.err = err
		group.done = true
		group.Unlock()
		go func() {
			time.Sleep(5 * time.Second)
			r.coalesceMu.Lock()
			delete(r.coalesceMap, key)
			r.coalesceMu.Unlock()
		}()
		return resp, err
	}
	r.coalesceMu.Unlock()

	group.Lock()
	for !group.done {
		group.Unlock()
		time.Sleep(10 * time.Millisecond)
		group.Lock()
	}
	result := group.result
	err := group.err
	group.Unlock()
	return result, err
}

func (r *AppsScriptRelay) doRequestSingle(targetURL string, headers map[string]string) (*http.Response, error) {
	ids := r.getScriptIDs()
	if len(ids) == 0 {
		return nil, fmt.Errorf("no Script ID configured")
	}
	return r.doRequest(ids[0], "GET", targetURL, headers, nil)
}

func (r *AppsScriptRelay) doRequest(scriptID, method, targetURL string, headers map[string]string, body []byte) (*http.Response, error) {
	cleanHeaders := cleanReqHeaders(headers)
	payload := map[string]interface{}{
		"m": method, "u": targetURL, "r": false, "h": cleanHeaders,
	}
	if len(body) > 0 {
		payload["b"] = base64.StdEncoding.EncodeToString(body)
		if ct, ok := headers["Content-Type"]; ok {
			payload["ct"] = ct
		} else if ct, ok := headers["content-type"]; ok {
			payload["ct"] = ct
		}
	}
	payload["k"] = r.cfg.AuthKey

	r.devMu.Lock()
	useDev := r.devAvailable
	r.devMu.Unlock()
	baseURL := fmt.Sprintf("https://script.google.com/macros/s/%s/exec", scriptID)
	if useDev {
		baseURL = fmt.Sprintf("https://script.google.com/macros/s/%s/dev", scriptID)
	}
	return r.sendSingle(baseURL, payload)
}

func cleanReqHeaders(headers map[string]string) map[string]string {
	skip := map[string]bool{
		"host": true, "connection": true, "content-length": true,
		"transfer-encoding": true, "proxy-connection": true,
		"proxy-authorization": true, "priority": true, "te": true,
		"accept-encoding": true,
	}
	cleaned := make(map[string]string)
	for k, v := range headers {
		if skip[strings.ToLower(k)] {
			continue
		}
		cleaned[k] = v
	}
	delete(cleaned, "Host")
	delete(cleaned, "host")
	return cleaned
}

func extractHost(u string) string {
	if idx := strings.Index(u, "://"); idx != -1 {
		u = u[idx+3:]
	}
	if idx := strings.IndexByte(u, '/'); idx != -1 {
		u = u[:idx]
	}
	if idx := strings.IndexByte(u, ':'); idx != -1 {
		u = u[:idx]
	}
	return u
}

// ── Core HTTP sender ─────────────────────────────────────────────────────

func (r *AppsScriptRelay) sendSingle(url string, payload map[string]interface{}) (*http.Response, error) {
	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Host = "script.google.com"
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %v", err)
	}

	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		location := resp.Header.Get("Location")
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if location == "" {
			return nil, fmt.Errorf("redirect without location")
		}
		req2, _ := http.NewRequest("POST", location, bytes.NewReader(jsonBody))
		req2.Host = "script.google.com"
		req2.Header.Set("Content-Type", "application/json")
		resp2, err := r.httpClient.Do(req2)
		if err != nil {
			return nil, err
		}
		resp = resp2
	}

	defer resp.Body.Close()

	var reader io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gzReader, err := gzip.NewReader(resp.Body)
		if err == nil {
			defer gzReader.Close()
			reader = gzReader
		}
	}
	respBytes, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read relay body: %v", err)
	}

	// Activate /dev if we see direct JSON
	if !r.devAvailable && strings.HasPrefix(strings.TrimSpace(string(respBytes)), "{") {
		r.devMu.Lock()
		r.devAvailable = true
		r.devMu.Unlock()
		log.Println("/dev endpoint activated (direct JSON response)")
	}

	raw, err := extractRelayJSON(respBytes)
	if err != nil {
		// Don't blacklist; just return the error as non-permanent
		return nil, fmt.Errorf("bad relay response: %v", err)
	}
	if eMsg, ok := raw["e"].(string); ok && eMsg != "" {
		return nil, fmt.Errorf("relay error: %s", eMsg)
	}

	status := 200
	if s, ok := raw["s"].(float64); ok {
		status = int(s)
	}

	respHeaders := make(http.Header)
	skipHeaders := map[string]bool{
		"transfer-encoding": true,
		"connection":        true,
		"keep-alive":        true,
		"content-length":    true,
	}
	if hRaw, ok := raw["h"].(map[string]interface{}); ok {
		for k, v := range hRaw {
			if skipHeaders[strings.ToLower(k)] {
				continue
			}
			for _, s := range headerValuesToStrings(v) {
				respHeaders.Add(k, s)
			}
		}
	}

	var respBody []byte
	if bStr, ok := raw["b"].(string); ok && bStr != "" {
		data, err := base64.StdEncoding.DecodeString(bStr)
		if err != nil {
			return nil, fmt.Errorf("base64 decode: %v", err)
		}

		// Relay-level gzip (gz flag, numeric)
		gzFlag := false
		if gzVal, ok := raw["gz"]; ok {
			switch v := gzVal.(type) {
			case float64:
				if v != 0 {
					gzFlag = true
				}
			case bool:
				gzFlag = v
			}
		}
		if gzFlag {
			if gr, err := gzip.NewReader(bytes.NewReader(data)); err == nil {
				if decomp, err := io.ReadAll(gr); err == nil {
					data = decomp
				}
				gr.Close()
			}
		}

		// Target Content-Encoding
		ce := respHeaders.Get("Content-Encoding")
		if ce != "" {
			if looksCompressed(data) {
				if decomp, err := decompressByEncoding(data, strings.ToLower(ce)); err == nil {
					data = decomp
					respHeaders.Del("Content-Encoding")
				}
			} else {
				respHeaders.Del("Content-Encoding")
			}
		}
		respBody = data
	}

	return &http.Response{
		Status:        http.StatusText(status),
		StatusCode:    status,
		Header:        respHeaders,
		ContentLength: int64(len(respBody)),
		Body:          io.NopCloser(bytes.NewReader(respBody)),
	}, nil
}

func looksCompressed(data []byte) bool {
	if len(data) < 2 {
		return false
	}
	if data[0] == 0x1f && data[1] == 0x8b {
		return true
	}
	if data[0] == 0xce {
		return true
	}
	if data[0] == 0x78 {
		return true
	}
	return false
}

func decompressByEncoding(data []byte, enc string) ([]byte, error) {
	switch enc {
	case "gzip", "x-gzip":
		r, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		defer r.Close()
		return io.ReadAll(r)
	case "deflate":
		zr, err := zlib.NewReader(bytes.NewReader(data))
		if err == nil {
			defer zr.Close()
			return io.ReadAll(zr)
		}
		fr := flate.NewReader(bytes.NewReader(data))
		defer fr.Close()
		return io.ReadAll(fr)
	case "br":
		r := brotli.NewReader(bytes.NewReader(data))
		return io.ReadAll(r)
	default:
		return nil, fmt.Errorf("unsupported encoding: %s", enc)
	}
}

// ── JSON extraction (robust, like Python) ───────────────────────────────

func extractRelayJSON(data []byte) (map[string]interface{}, error) {
	// 1. Direct JSON
	var raw map[string]interface{}
	if json.Unmarshal(data, &raw) == nil {
		return raw, nil
	}

	text := string(data)

	// 2. IFRAME_SANDBOX extraction (exactly like Python)
	if start := strings.Index(text, `goog.script.init("`); start != -1 {
		start += len(`goog.script.init("`)
		if end := strings.Index(text[start:], `", "", undefined`); end != -1 {
			encoded := text[start : start+end]
			decoded := unescapeJSString(encoded)
			var payload map[string]interface{}
			if json.Unmarshal([]byte(decoded), &payload) == nil {
				if uh, ok := payload["userHtml"]; ok {
					if s, ok := uh.(string); ok {
						return extractRelayJSON([]byte(s))
					}
				}
			}
		}
	}

	// 3. Fallback: find any JSON object inside the text
	re := regexp.MustCompile(`\{.*\}`)
	if m := re.FindString(text); m != "" {
		var raw2 map[string]interface{}
		if json.Unmarshal([]byte(m), &raw2) == nil {
			return raw2, nil
		}
	}

	return nil, fmt.Errorf("no JSON found")
}

func unescapeJSString(s string) string {
	// Order: hex -> double backslash -> forward slash (Python order)
	reHex := regexp.MustCompile(`\\x([0-9a-fA-F]{2})`)
	out := reHex.ReplaceAllStringFunc(s, func(m string) string {
		var b byte
		fmt.Sscanf(m[2:], "%x", &b)
		return string(b)
	})
	out = strings.ReplaceAll(out, "\\\\", "\\")
	out = strings.ReplaceAll(out, "\\/", "/")
	return out
}

func headerValuesToStrings(val interface{}) []string {
	switch v := val.(type) {
	case string:
		return []string{v}
	case []interface{}:
		out := make([]string, len(v))
		for i, item := range v {
			out[i] = fmt.Sprintf("%v", item)
		}
		return out
	default:
		return []string{fmt.Sprintf("%v", v)}
	}
}

// ── Health Check ───────────────────────────────────────────────────────

func (r *AppsScriptRelay) StartHealthCheck(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			for _, id := range r.collectScriptIDs() {
				_, err := r.doRequest(id, "GET", "http://example.com/", nil, nil)
				if err == nil {
					r.blacklistMu.Lock()
					delete(r.blacklist, id)
					r.blacklistMu.Unlock()
				} else if isPermanentError(err) {
					r.blacklistScript(id)
				}
			}
		}
	}()
}

// ── TestDeployment ─────────────────────────────────────────────────────

type TestDeploymentResult struct {
	OK       bool   `json:"ok"`
	Status   int    `json:"status"`
	Body     string `json:"body"`
	Error    string `json:"error,omitempty"`
	PingMs   int64  `json:"ping_ms"`
	TargetIP string `json:"target_ip,omitempty"`
}

func TestDeployment(googleIP, frontDomain, scriptID, authKey string) TestDeploymentResult {
	cfg := config.Config{
		GoogleIP: googleIP, FrontDomain: frontDomain,
		ScriptID: scriptID, AuthKey: authKey, VerifySSL: true,
	}
	relay := NewRelay(cfg)
	start := time.Now()
	resp, err := relay.Do("GET", "https://api.ipify.org/?format=json", nil, nil)
	if err != nil {
		return TestDeploymentResult{Error: err.Error(), PingMs: time.Since(start).Milliseconds()}
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	r := TestDeploymentResult{
		Status: resp.StatusCode, Body: string(b),
		PingMs: time.Since(start).Milliseconds(),
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		r.OK = true
		var ip map[string]interface{}
		if json.Unmarshal(b, &ip) == nil {
			if ipStr, ok := ip["ip"].(string); ok {
				r.TargetIP = ipStr
			}
		}
	} else {
		r.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
	}
	return r
}

func CleanHeaders(h http.Header) map[string]string {
	out := make(map[string]string)
	skip := map[string]bool{
		"host": true, "connection": true, "content-length": true,
		"transfer-encoding": true, "proxy-connection": true,
		"proxy-authorization": true, "priority": true, "te": true,
	}
	for k, v := range h {
		if skip[strings.ToLower(k)] {
			continue
		}
		if len(v) > 0 {
			out[k] = v[0]
		}
	}
	return out
}