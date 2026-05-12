package main

import (
	"fmt"
	"net"
	"time"
)

func pingGoogle(ip string) (float64, error) {
	start := time.Now()
	conn, err := net.DialTimeout("tcp", ip+":80", 1*time.Second)
	if err != nil {
		return 0, err
	}
	conn.Close()
	return float64(time.Since(start).Milliseconds()), nil
}

func parseInt(s string) (int, error) {
	var i int
	_, err := fmt.Sscanf(s, "%d", &i)
	return i, err
}