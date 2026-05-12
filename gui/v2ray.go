package gui

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"mhrv-go/config"
	"mhrv-go/proxy"
)

// RegisterV2RayHandlers registers all V2Ray related endpoints
func RegisterV2RayHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/api/v2ray/add", handleV2RayAdd)
	mux.HandleFunc("/api/v2ray/delete", handleV2RayDelete)
	mux.HandleFunc("/api/v2ray/ping", handleV2RayPing)
	mux.HandleFunc("/api/v2ray/list", handleV2RayList)
	mux.HandleFunc("/api/v2ray/connect", handleV2RayConnect)
}

func handleV2RayConnect(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "missing id", 400)
		return
	}

	var targetConfig *config.V2RayConfig
	for _, c := range cfg.V2RayConfigs {
		if c.ID == id {
			targetConfig = &c
			break
		}
	}

	if targetConfig == nil {
		http.Error(w, "config not found", 404)
		return
	}

	if proxyServer == nil {
		var err error
		proxyServer, err = proxy.NewServer(cfg)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
	}

	if err := proxyServer.ConnectV2Ray(*targetConfig); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"status": "connecting"})
}

func handleV2RayAdd(w http.ResponseWriter, r *http.Request) {
	var newConfig config.V2RayConfig
	if err := json.NewDecoder(r.Body).Decode(&newConfig); err != nil {
		http.Error(w, "bad json", 400)
		return
	}
	if newConfig.ID == "" {
		newConfig.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	
	cfg.V2RayConfigs = append(cfg.V2RayConfigs, newConfig)
	config.SaveMode(cfg)
	
	json.NewEncoder(w).Encode(map[string]string{"status": "added", "id": newConfig.ID})
}

func handleV2RayDelete(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "missing id", 400)
		return
	}
	
	newConfigs := []config.V2RayConfig{}
	for _, c := range cfg.V2RayConfigs {
		if c.ID != id {
			newConfigs = append(newConfigs, c)
		}
	}
	cfg.V2RayConfigs = newConfigs
	config.SaveMode(cfg)
	
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

func handleV2RayPing(w http.ResponseWriter, r *http.Request) {
	// This is a simple TCP ping for the example. 
	// In a real implementation, you would parse the config to get the address.
	address := r.URL.Query().Get("address")
	if address == "" {
		http.Error(w, "missing address", 400)
		return
	}

	start := time.Now()
	conn, err := net.DialTimeout("tcp", address, 2*time.Second)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "error", "ping": -1, "message": err.Error()})
		return
	}
	conn.Close()
	elapsed := time.Since(start).Milliseconds()
	
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "ping": elapsed})
}

func handleV2RayList(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(cfg.V2RayConfigs)
}
