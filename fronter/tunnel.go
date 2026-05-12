package fronter

import (
	"crypto/tls"
	"io"
	"net"
	"sync"
	"time"
)

func SNITunnel(clientConn net.Conn, target string, googleIP, frontDomain string) error {
	serverTLSConfig := &tls.Config{
		ServerName: frontDomain,
	}
	dialer := net.Dialer{Timeout: 10 * time.Second}
	googleConn, err := dialer.Dial("tcp", net.JoinHostPort(googleIP, "443"))
	if err != nil {
		return err
	}
	defer googleConn.Close()

	tlsConn := tls.Client(googleConn, serverTLSConfig)
	if err := tlsConn.Handshake(); err != nil {
		return err
	}
	defer tlsConn.Close()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		io.Copy(tlsConn, clientConn)
		tlsConn.CloseWrite()
	}()
	go func() {
		defer wg.Done()
		io.Copy(clientConn, tlsConn)
		clientConn.(*net.TCPConn).CloseWrite()
	}()
	wg.Wait()
	return nil
}