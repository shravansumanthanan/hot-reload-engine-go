package proxy

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Proxy struct {
	targetURL *url.URL
	address   string

	mu      sync.Mutex
	clients map[chan struct{}]struct{}
	server  *http.Server
}

func New(address, targetAddr string) (*Proxy, error) {
	u, err := url.Parse(targetAddr)
	if err != nil {
		return nil, err
	}
	return &Proxy{
		targetURL: u,
		address:   address,
		clients:   make(map[chan struct{}]struct{}),
	}, nil
}

func (p *Proxy) Start() error {
	rp := httputil.NewSingleHostReverseProxy(p.targetURL)
	rp.ModifyResponse = p.modifyResponse
	rp.ErrorHandler = p.errorHandler

	mux := http.NewServeMux()
	mux.HandleFunc("/__hotreload_sse", p.sseHandler)
	mux.Handle("/", rp)

	slog.Info("Starting Live-Reload Proxy", "listen", p.address, "target", p.targetURL.String())

	server := &http.Server{
		Addr:              p.address,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      0, // 0 = no timeout for SSE long-poll connections
		IdleTimeout:       120 * time.Second,
	}

	p.mu.Lock()
	p.server = server
	p.mu.Unlock()

	return server.ListenAndServe()
}

// Shutdown gracefully shuts down the proxy server, closing all SSE connections.
func (p *Proxy) Shutdown(ctx context.Context) error {
	p.mu.Lock()
	s := p.server
	p.mu.Unlock()

	if s == nil {
		return nil
	}
	slog.Info("Shutting down proxy server")
	return s.Shutdown(ctx)
}

// WaitForTarget polls the target address until it's accepting connections or
// the context is cancelled. Returns true if the target became ready.
func (p *Proxy) WaitForTarget(ctx context.Context) bool {
	host := p.targetURL.Host // e.g. "127.0.0.1:8081"
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
			conn, err := net.DialTimeout("tcp", host, 100*time.Millisecond)
			if err == nil {
				conn.Close()
				return true
			}
		}
	}
}

// errorHandler serves a friendly auto-retry page when the backend is
// unavailable (e.g. during a rebuild). The page automatically retries
// until the server comes back up.
func (p *Proxy) errorHandler(w http.ResponseWriter, r *http.Request, err error) {
	slog.Debug("Proxy backend unavailable, serving retry page", "err", err)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusBadGateway)
	fmt.Fprint(w, retryPage)
}

func (p *Proxy) BroadcastReload() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for c := range p.clients {
		select {
		case c <- struct{}{}:
		default:
		}
	}
}

func (p *Proxy) sseHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	// Restrict CORS to the proxy's own localhost address instead of wildcard.
	// Since the injected script is served by the proxy itself this is same-origin,
	// but an explicit restriction prevents other tabs from subscribing.
	origin := "http://localhost"
	if strings.HasPrefix(p.address, ":") {
		origin = "http://localhost" + p.address
	}
	w.Header().Set("Access-Control-Allow-Origin", origin)

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	ch := make(chan struct{}, 1)
	p.mu.Lock()
	p.clients[ch] = struct{}{}
	p.mu.Unlock()

	defer func() {
		p.mu.Lock()
		delete(p.clients, ch)
		p.mu.Unlock()
	}()

	fmt.Fprintf(w, "data: connected\n\n")
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			// Client disconnected.
			return
		case <-ch:
			fmt.Fprintf(w, "data: reload\n\n")
			flusher.Flush()
			// Re-arm the channel so the next reload can be received.
			// The channel is buffered (size 1), so we just drain it and
			// keep looping — the connection stays open for future reloads.
		}
	}
}

const injectedScript = `
<script>
	(function() {
		let evtSource = new EventSource("/__hotreload_sse");
		evtSource.onmessage = function(event) {
			if (event.data === "reload") {
				window.location.reload();
			}
		};
		evtSource.onerror = function() {
			evtSource.close();
			let attempts = 0;
			let checkInterval = setInterval(async () => {
				attempts++;
				if (attempts > 30) { clearInterval(checkInterval); return; }
				try {
					const res = await fetch(window.location.href, { cache: "no-store", method: "HEAD" });
					if (res.ok) {
						clearInterval(checkInterval);
						window.location.reload();
					}
				} catch (e) {}
			}, 500);
		};
	})();
</script>
`

// retryPage is served when the backend is unavailable (during rebuilds).
// It auto-retries every 500ms until the server comes back, then reloads.
const retryPage = `<!DOCTYPE html>
<html>
<head><title>Reloading...</title></head>
<body style="font-family:sans-serif;display:flex;align-items:center;justify-content:center;height:100vh;margin:0;background:#f5f5f5;color:#333">
<div style="text-align:center">
<h2>Rebuilding...</h2>
<p>The server is restarting. This page will reload automatically.</p>
</div>
<script>
(function(){
	let attempts = 0;
	let iv = setInterval(async () => {
		attempts++;
		if (attempts > 60) { clearInterval(iv); return; }
		try {
			const r = await fetch(window.location.href, {cache:"no-store",method:"HEAD"});
			if (r.ok) { clearInterval(iv); window.location.reload(); }
		} catch(e){}
	}, 500);
})();
</script>
</body>
</html>`

func (p *Proxy) modifyResponse(resp *http.Response) error {
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(strings.ToLower(contentType), "text/html") {
		return nil
	}

	var bodyReader io.ReadCloser = resp.Body
	var err error

	isGzip := resp.Header.Get("Content-Encoding") == "gzip"
	if isGzip {
		bodyReader, err = gzip.NewReader(resp.Body)
		if err != nil {
			slog.Warn("Failed to decompress gzip response, skipping injection", "err", err)
			return nil // Don't fail the request, just skip injection
		}
		defer bodyReader.Close()
	}

	body, err := io.ReadAll(bodyReader)
	if err != nil {
		slog.Warn("Failed to read response body, skipping injection", "err", err)
		return nil // Don't fail the request
	}
	_ = bodyReader.Close()
	_ = resp.Body.Close()

	bodyStr := string(body)
	idx := strings.LastIndex(strings.ToLower(bodyStr), "</body>")
	if idx != -1 {
		bodyStr = bodyStr[:idx] + injectedScript + bodyStr[idx:]
	} else {
		bodyStr += injectedScript
	}

	var newBodyBuf bytes.Buffer
	if isGzip {
		gz := gzip.NewWriter(&newBodyBuf)
		_, err = gz.Write([]byte(bodyStr))
		if err != nil {
			slog.Warn("Failed to compress response, sending uncompressed", "err", err)
			// Fall back to uncompressed
			newBodyBuf.Reset()
			newBodyBuf.WriteString(bodyStr)
			resp.Header.Del("Content-Encoding")
		} else {
			_ = gz.Close()
		}
	} else {
		newBodyBuf.WriteString(bodyStr)
	}

	resp.Body = io.NopCloser(&newBodyBuf)
	resp.Header.Set("Content-Length", strconv.Itoa(newBodyBuf.Len()))

	return nil
}
