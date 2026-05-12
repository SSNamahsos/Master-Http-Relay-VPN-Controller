package proxy

import (
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"mhrv-go/cert"
	"mhrv-go/config"
	"mhrv-go/fronter"
	"golang.org/x/net/proxy"
)

type Server struct {
	config      config.Config
	relay       *fronter.AppsScriptRelay
	listener    net.Listener
	stopCh      chan struct{}
	stats       Stats
	mu          sync.Mutex
	socksDialer proxy.Dialer // برای اتصال از طریق SOCKS5 (حالت github)
}

type Stats struct {
	Requests uint64 `json:"requests"`
	Active   uint64 `json:"active"`
	Bytes    uint64 `json:"bytes"`
}

func NewServer(cfg config.Config) (*Server, error) {
	if cfg.Mode == "apps_script" {
		if err := cert.EnsureCA(); err != nil {
			return nil, fmt.Errorf("failed to ensure CA: %v", err)
		}
	}

	s := &Server{
		config:  cfg,
		stopCh:  make(chan struct{}),
	}

	if cfg.Mode == "github" {
		// ساخت dialer برای اتصال از طریق SOCKS5 محلی Xray
		socksAddr := fmt.Sprintf("127.0.0.1:%d", cfg.Xray.Port)
		dialer, err := proxy.SOCKS5("tcp", socksAddr, nil, nil)
		if err != nil {
			return nil, fmt.Errorf("socks5 dialer: %v", err)
		}
		s.socksDialer = dialer
	} else {
		relay := fronter.NewRelay(cfg)
		relay.StartHealthCheck(5 * time.Second)
		s.relay = relay
	}

	return s, nil
}

func (s *Server) Start() error {
	if s.config.LanSharing {
		s.config.ListenHost = "0.0.0.0"
	}
	addr := fmt.Sprintf("%s:%d", s.config.ListenHost, s.config.ListenPort)
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	s.listener = l
	log.Printf("Proxy listening on %s", addr)

	go func() {
		<-s.stopCh
		l.Close()
	}()

	for {
		conn, err := l.Accept()
		if err != nil {
			select {
			case <-s.stopCh:
				return nil
			default:
				continue
			}
		}
		go s.handleConn(conn)
	}
}

func (s *Server) Stop() {
	close(s.stopCh)
	if s.listener != nil {
		s.listener.Close()
	}
}

func (s *Server) Stats() Stats {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stats
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()
	s.mu.Lock()
	s.stats.Requests++
	s.stats.Active++
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.stats.Active--
		s.mu.Unlock()
	}()

	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		return
	}
	reqStr := string(buf[:n])
	lines := strings.SplitN(reqStr, "\r\n", 2)
	if len(lines) < 1 {
		return
	}
	firstLine := strings.SplitN(lines[0], " ", 3)
	if len(firstLine) < 3 {
		return
	}
	method := firstLine[0]
	url := firstLine[1]

	clientAddr := conn.RemoteAddr().String()
	var host string
	if method == "CONNECT" {
		host, _, _ = net.SplitHostPort(url)
		if host == "" {
			host = url
		}
		log.Printf("from %s accepted //%s:443 [https]", clientAddr, host)
	} else {
		if len(lines) > 1 {
			for _, hdr := range strings.Split(lines[1], "\r\n") {
				if strings.HasPrefix(strings.ToLower(hdr), "host:") {
					host = strings.TrimSpace(hdr[len("host:"):])
					break
				}
			}
		}
		if host == "" {
			host = "unknown"
		}
		log.Printf("from %s accepted //%s:80 [http]", clientAddr, host)
	}

	if method == "CONNECT" {
		s.handleConnect(conn, url)
	} else {
		s.handleHTTP(conn, buf[:n])
	}
}

// dial یک اتصال مستقیم یا از طریق SOCKS5 (اگر حالت github فعال باشد) برقرار می‌کند.
func (s *Server) dial(network, address string) (net.Conn, error) {
	if s.socksDialer != nil {
		return s.socksDialer.Dial(network, address)
	}
	return net.Dial(network, address)
}