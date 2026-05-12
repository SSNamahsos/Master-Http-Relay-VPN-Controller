package proxy

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"

	"mhrv-go/config"
)

var (
	v2rayCmd   *exec.Cmd
	v2rayMutex sync.Mutex
)

func (s *Server) ConnectV2Ray(cfg config.V2RayConfig) error {
	v2rayMutex.Lock()
	defer v2rayMutex.Unlock()

	if v2rayCmd != nil && v2rayCmd.Process != nil && v2rayCmd.ProcessState == nil {
		v2rayCmd.Process.Kill()
	}

	binPath := filepath.Join(config.GetDocsDir(), "xray", "xray.exe")
	if runtime.GOOS != "windows" {
		binPath = filepath.Join(config.GetDocsDir(), "xray", "xray")
	}

	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		return fmt.Errorf("xray binary not found. please download it in github codespaces section first")
	}

	configPath, err := generateXrayConfigFromV2Ray(cfg, s.config.Socks5Port)
	if err != nil {
		return err
	}

	v2rayCmd = exec.Command(binPath, "run", "-config", configPath)
	if err := v2rayCmd.Start(); err != nil {
		return err
	}

	return nil
}

func generateXrayConfigFromV2Ray(v2cfg config.V2RayConfig, socksPort int) (string, error) {
	// This is a simplified implementation. 
	// Real implementation would parse vmess://, ss://, etc.
	// For now, we'll assume the Content is already a JSON config or we treat it as a server address for a simple socks outbound.
	
	var xrayConfig map[string]interface{}
	
	// If it's already JSON, use it
	if err := json.Unmarshal([]byte(v2cfg.Content), &xrayConfig); err != nil {
		// Not JSON, assume it's a URL or address and try to build a simple config
		// This is just a placeholder logic
		xrayConfig = map[string]interface{}{
			"inbounds": []map[string]interface{}{
				{
					"port":     socksPort,
					"protocol": "socks",
					"settings": map[string]interface{}{
						"auth": "noauth",
						"udp":  true,
					},
				},
			},
			"outbounds": []map[string]interface{}{
				{
					"protocol": "vmess", // placeholder
					"settings": map[string]interface{}{
						"vnext": []map[string]interface{}{
							{
								"address": "127.0.0.1",
								"port":    443,
								"users": []map[string]interface{}{
									{"id": v2cfg.Content},
								},
							},
						},
					},
				},
			},
		}
	}

	jsonBytes, _ := json.MarshalIndent(xrayConfig, "", "  ")
	configDir := filepath.Join(config.GetDocsDir(), "v2ray")
	os.MkdirAll(configDir, 0755)
	configPath := filepath.Join(configDir, "config.json")
	if err := os.WriteFile(configPath, jsonBytes, 0644); err != nil {
		return "", err
	}
	return configPath, nil
}
