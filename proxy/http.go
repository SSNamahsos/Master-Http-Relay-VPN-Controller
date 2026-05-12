package proxy

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"

	"mhrv-go/fronter"
)

func (s *Server) handleHTTP(clientConn net.Conn, initialData []byte) {
	reader := bufio.NewReader(io.MultiReader(strings.NewReader(string(initialData)), clientConn))
	req, err := http.ReadRequest(reader)
	if err != nil {
		return
	}

	// ساخت URL مطلق
	targetURL := req.URL.String()
	if !req.URL.IsAbs() {
		host := req.Host
		scheme := "http"
		targetURL = fmt.Sprintf("%s://%s%s", scheme, host, req.URL.RequestURI())
	}

	headers := fronter.CleanHeaders(req.Header)

	if s.config.Mode == "github" {
		// در حالت GitHub، درخواست HTTP را مستقیماً از طریق SOCKS5 ارسال کن
		host := req.Host
		if !strings.Contains(host, ":") {
			host += ":80"
		}
		remote, err := s.dial("tcp", host)
		if err != nil {
			clientConn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
			return
		}
		defer remote.Close()
		req.Write(remote)
		io.Copy(clientConn, remote)
		return
	}

	// خواندن body در صورت وجود
	var body []byte
	if req.Body != nil {
		body, _ = io.ReadAll(req.Body)
	}

	resp, err := s.relay.Do(req.Method, targetURL, headers, body)
	if err != nil {
		clientConn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
		return
	}
	defer resp.Body.Close()
	resp.Write(clientConn)
}