package proxy

import (
	"bufio"
	"crypto/tls"
	"io"
	"log"
	"net"
	"net/http"
	"strings"

	"mhrv-go/fronter"
	"mhrv-go/mitm"
)

var sniRewriteSuffixes = []string{
	".youtube.com", ".youtu.be", ".youtube-nocookie.com", ".ytimg.com",
	".ggpht.com", ".gvt1.com", ".gvt2.com", ".doubleclick.net",
	".googlesyndication.com", ".googleadservices.com", ".google-analytics.com",
	".googletagmanager.com", ".googletagservices.com", ".fonts.googleapis.com",
}

func isSNIRewriteDomain(host string) bool {
	h := strings.ToLower(host)
	for _, suffix := range sniRewriteSuffixes {
		if strings.HasSuffix(h, suffix) || h == suffix[1:] {
			return true
		}
	}
	return false
}

func isGoogleDomain(host string) bool {
	h := strings.ToLower(host)
	if h == "google.com" || h == "gstatic.com" || h == "googleapis.com" {
		return true
	}
	for _, suffix := range []string{".google.com", ".google.co", ".googleapis.com", ".gstatic.com", ".googleusercontent.com"} {
		if strings.HasSuffix(h, suffix) {
			return true
		}
	}
	return false
}

func (s *Server) handleConnect(clientConn net.Conn, target string) {
	host, port, _ := net.SplitHostPort(target)
	if port == "" {
		port = "443"
	}

	clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))

	switch s.config.Mode {
	case "github":
		// در حالت GitHub، همه ترافیک مستقیماً از طریق SOCKS5 (Xray) منتقل می‌شود.
		s.handleRawTCP(clientConn, host, port)
	case "google_only":
		if isGoogleDomain(host) || isSNIRewriteDomain(host) {
			s.handleSNIRewriteMITM(clientConn, host, port)
		} else {
			s.handleRawTCP(clientConn, host, port)
		}
	case "apps_script":
		if isSNIRewriteDomain(host) {
			s.handleSNIRewriteMITM(clientConn, host, port)
		} else if isGoogleDomain(host) {
			s.handleRawTCP(clientConn, host, port)
		} else {
			s.handleMITMRelay(clientConn, host, port)
		}
	default:
		s.handleRawTCP(clientConn, host, port)
	}
}

func (s *Server) handleRawTCP(clientConn net.Conn, host, port string) {
	remote, err := s.dial("tcp", net.JoinHostPort(host, port))
	if err != nil {
		log.Printf("failed to dial %s:%s: %v", host, port, err)
		return
	}
	defer remote.Close()
	go io.Copy(remote, clientConn)
	io.Copy(clientConn, remote)
}

func (s *Server) handleSNIRewriteMITM(clientConn net.Conn, host, port string) {
	certManager := mitm.NewCertManager()
	tlsConfig := certManager.GetTLSConfig(host)
	clientTLS := tls.Server(clientConn, tlsConfig)
	defer clientTLS.Close()

	reader := bufio.NewReader(clientTLS)
	for {
		req, err := http.ReadRequest(reader)
		if err != nil {
			return
		}

		outTLS, err := tls.Dial("tcp", net.JoinHostPort(s.config.GoogleIP, "443"), &tls.Config{
			ServerName: s.config.FrontDomain,
		})
		if err != nil {
			return
		}
		defer outTLS.Close()

		if err := req.Write(outTLS); err != nil {
			return
		}
		resp, err := http.ReadResponse(bufio.NewReader(outTLS), req)
		if err != nil {
			return
		}
		defer resp.Body.Close()
		resp.Write(clientTLS)
	}
}

func (s *Server) handleMITMRelay(clientConn net.Conn, host, port string) {
	certManager := mitm.NewCertManager()
	tlsConfig := certManager.GetTLSConfig(host)
	clientTLS := tls.Server(clientConn, tlsConfig)
	defer clientTLS.Close()

	reader := bufio.NewReader(clientTLS)
	for {
		req, err := http.ReadRequest(reader)
		if err != nil {
			return
		}

		targetURL := "https://" + host
		if port != "443" {
			targetURL += ":" + port
		}
		targetURL += req.URL.RequestURI()

		headers := fronter.CleanHeaders(req.Header)
		delete(headers, "Accept-Encoding")
		delete(headers, "accept-encoding")

		var body []byte
		if req.Body != nil {
			body, _ = io.ReadAll(req.Body)
		}

		resp, err := s.relay.Do(req.Method, targetURL, headers, body)
		if err != nil {
			log.Printf("RELAY ERROR for %s: %v", targetURL, err)
			errResp := []byte("HTTP/1.1 502 Bad Gateway\r\nContent-Length: 0\r\nConnection: keep-alive\r\n\r\n")
			clientTLS.Write(errResp)
			continue
		}

		resp.Header.Set("Connection", "keep-alive")
		if err := resp.Write(clientTLS); err != nil {
			return
		}
		resp.Body.Close()
	}
}