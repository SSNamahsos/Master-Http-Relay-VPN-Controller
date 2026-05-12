package gui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"mhrv-go/config"
)

// ---------- state ----------
type gitHubState struct {
	Authenticated bool                 `json:"authenticated"`
	Username      string               `json:"username"`
	Forked        bool                 `json:"forked"`
	ForkURL       string               `json:"forkUrl"`
	Status        string               `json:"status"` // idle, forking, creating, ready, error
	LastError     string               `json:"lastError,omitempty"`
	Codespace     gitHubCodespaceState `json:"codespace,omitempty"`
	mu            sync.Mutex
}

type gitHubCodespaceState struct {
	Name      string    `json:"name"`
	WebURL    string    `json:"webUrl"`
	VLESS     string    `json:"vless"`
	Status    string    `json:"status"` // creating, ready, error
	UpdatedAt time.Time `json:"updatedAt"`
}

var (
	ghState gitHubState
	ghMu    sync.Mutex
)

// RegisterGitHubHandlers registers all GitHub related endpoints
func RegisterGitHubHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/api/github/login", handleGitHubLogin)
	mux.HandleFunc("/api/github/logout", handleGitHubLogout)
	mux.HandleFunc("/api/github/fork", handleGitHubFork)
	mux.HandleFunc("/api/github/create", handleGitHubCreate)
	mux.HandleFunc("/api/github/stop", handleGitHubStop)
	mux.HandleFunc("/api/github/delete", handleGitHubDelete)
	mux.HandleFunc("/api/github/status", handleGitHubStatus)
	mux.HandleFunc("/api/xray/start", handleXrayStart)
	mux.HandleFunc("/api/xray/stop", handleXrayStop)
}

// ---------- helpers ----------
func ghAPIRequest(method, url, token string, body interface{}) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		reqBody = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	client := &http.Client{Timeout: 30 * time.Second}
	return client.Do(req)
}

func startOAuthServer() (string, <-chan string, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", nil, err
	}
	port := listener.Addr().(*net.TCPAddr).Port
	redirectURI := fmt.Sprintf("http://127.0.0.1:%d/callback", port)
	codeCh := make(chan string, 1)
	mux := http.NewServeMux()
	server := &http.Server{Handler: mux}
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code != "" {
			codeCh <- code
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte("<html><body style='font-family:sans-serif;text-align:center;padding-top:50px;'><h2>Authentication successful!</h2><p>You can close this window and return to the app.</p></body></html>"))
		} else {
			w.Write([]byte("Authorization failed."))
		}
		go func() {
			time.Sleep(1 * time.Second)
			server.Close()
		}()
	})
	go server.Serve(listener)
	return redirectURI, codeCh, nil
}

func exchangeCodeForToken(code, redirectURI string) (string, error) {
	clientID := cfg.GitHub.ClientID
	clientSecret := cfg.GitHub.ClientSecret
	if clientID == "" || clientSecret == "" {
		return "", fmt.Errorf("GitHub ClientID or ClientSecret not configured")
	}

	data := map[string]string{
		"client_id":     clientID,
		"client_secret": clientSecret,
		"code":          code,
		"redirect_uri":  redirectURI,
	}
	jsonData, _ := json.Marshal(data)
	req, _ := http.NewRequest("POST", "https://github.com/login/oauth/access_token", bytes.NewReader(jsonData))
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
		Description string `json:"error_description"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	if result.Error != "" {
		return "", fmt.Errorf("oauth error: %s (%s)", result.Error, result.Description)
	}
	return result.AccessToken, nil
}

func handleGitHubLogin(w http.ResponseWriter, r *http.Request) {
	if cfg.GitHub.ClientID == "" || cfg.GitHub.ClientSecret == "" {
		http.Error(w, "GitHub Client ID or Secret not configured in settings", 400)
		return
	}

	redirectURI, codeCh, err := startOAuthServer()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	authURL := fmt.Sprintf("https://github.com/login/oauth/authorize?client_id=%s&scope=repo,user,codespace&redirect_uri=%s", cfg.GitHub.ClientID, redirectURI)
	openURL(authURL)

	select {
	case code := <-codeCh:
		token, err := exchangeCodeForToken(code, redirectURI)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		cfg.GitHub.Enabled = true
		cfg.GitHub.Token = token
		config.SaveMode(cfg)

		resp, err := ghAPIRequest("GET", "https://api.github.com/user", token, nil)
		if err == nil && resp.StatusCode == 200 {
			var user struct{ Login string `json:"login"` }
			json.NewDecoder(resp.Body).Decode(&user)
			resp.Body.Close()
			ghMu.Lock()
			ghState.Authenticated = true
			ghState.Username = user.Login
			ghMu.Unlock()
			broadcastLog(fmt.Sprintf("[GitHub] Authenticated as %s\n", user.Login))
		}

		json.NewEncoder(w).Encode(map[string]string{"status": "authenticated", "username": ghState.Username})
	case <-time.After(5 * time.Minute):
		http.Error(w, "Authentication timed out", 408)
	}
}

func handleGitHubLogout(w http.ResponseWriter, r *http.Request) {
	cfg.GitHub.Token = ""
	config.SaveMode(cfg)
	ghMu.Lock()
	ghState = gitHubState{}
	ghMu.Unlock()
	broadcastLog("[GitHub] Logged out.\n")
	json.NewEncoder(w).Encode(map[string]string{"status": "logged_out"})
}

func handleGitHubFork(w http.ResponseWriter, r *http.Request) {
	token := cfg.GitHub.Token
	if token == "" {
		http.Error(w, "Not authenticated", 401)
		return
	}

	// Check if already forked
	resp, _ := ghAPIRequest("GET", fmt.Sprintf("https://api.github.com/repos/%s/%s", ghState.Username, cfg.GitHub.RepoName), token, nil)
	if resp != nil && resp.StatusCode == 200 {
		ghMu.Lock()
		ghState.Forked = true
		ghState.ForkURL = fmt.Sprintf("https://github.com/%s/%s", ghState.Username, cfg.GitHub.RepoName)
		ghMu.Unlock()
		json.NewEncoder(w).Encode(map[string]bool{"forked": true})
		return
	}

	broadcastLog("[GitHub] Forking repository " + cfg.GitHub.RepoOwner + "/" + cfg.GitHub.RepoName + "...\n")
	forkResp, err := ghAPIRequest("POST", fmt.Sprintf("https://api.github.com/repos/%s/%s/forks", cfg.GitHub.RepoOwner, cfg.GitHub.RepoName), token, nil)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer forkResp.Body.Close()

	if forkResp.StatusCode == 202 {
		ghMu.Lock()
		ghState.Forked = true
		ghState.ForkURL = fmt.Sprintf("https://github.com/%s/%s", ghState.Username, cfg.GitHub.RepoName)
		ghMu.Unlock()
		broadcastLog("[GitHub] Repository fork initiated.\n")
		json.NewEncoder(w).Encode(map[string]bool{"forked": true})
	} else {
		body, _ := io.ReadAll(forkResp.Body)
		http.Error(w, fmt.Sprintf("Fork failed: %s", string(body)), forkResp.StatusCode)
	}
}

func handleGitHubCreate(w http.ResponseWriter, r *http.Request) {
	if cfg.GitHub.Token == "" {
		http.Error(w, "GitHub not authenticated", 400)
		return
	}
	owner := ghState.Username
	if owner == "" {
		http.Error(w, "User information missing", 400)
		return
	}

	ghMu.Lock()
	ghState.Status = "creating"
	ghState.LastError = ""
	ghMu.Unlock()

	go func() {
		broadcastLog("[GitHub] Creating Codespace for " + owner + "/" + cfg.GitHub.RepoName + "...\n")
		apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/codespaces", owner, cfg.GitHub.RepoName)
		reqBody := map[string]interface{}{
			"machine":      cfg.GitHub.Machine,
			"ref":          cfg.GitHub.Branch,
			"display_name": "mihani-g2ray",
		}
		resp, err := ghAPIRequest("POST", apiURL, cfg.GitHub.Token, reqBody)
		if err != nil {
			setGHError(err.Error())
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 300 {
			var apiErr map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&apiErr)
			setGHError(fmt.Sprintf("API Error %d: %v", resp.StatusCode, apiErr))
			return
		}

		var cs struct {
			Name   string `json:"name"`
			WebURL string `json:"web_url"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&cs); err != nil {
			setGHError(err.Error())
			return
		}

		// Wait for codespace to be ready
		for i := 0; i < 20; i++ {
			time.Sleep(5 * time.Second)
			checkURL := fmt.Sprintf("https://api.github.com/user/codespaces/%s", cs.Name)
			cResp, err := ghAPIRequest("GET", checkURL, cfg.GitHub.Token, nil)
			if err == nil && cResp.StatusCode == 200 {
				var status struct {
					State string `json:"state"`
				}
				json.NewDecoder(cResp.Body).Decode(&status)
				cResp.Body.Close()
				if status.State == "Available" {
					break
				}
				broadcastLog(fmt.Sprintf("[GitHub] Codespace status: %s...\n", status.State))
			}
		}

		sni := fmt.Sprintf("%s-443.app.github.dev", cs.Name)
		vless := fmt.Sprintf("vless://550e8400-e29b-41d4-a716-446655440000@94.130.50.12:443?encryption=none&security=tls&type=xhttp&mode=packet-up&sni=%s&path=%%2F#ghtun", sni)

		ghMu.Lock()
		ghState.Codespace = gitHubCodespaceState{
			Name:   cs.Name,
			WebURL: cs.WebURL,
			VLESS:  vless,
			Status: "ready",
		}
		ghState.Status = "ready"
		ghMu.Unlock()
		broadcastLog("[GitHub] Codespace ready: " + cs.Name + "\n")
	}()

	json.NewEncoder(w).Encode(map[string]string{"status": "creating"})
}

func setGHError(msg string) {
	ghMu.Lock()
	ghState.Status = "error"
	ghState.LastError = msg
	ghMu.Unlock()
	broadcastLog(fmt.Sprintf("[GitHub] ERROR: %s\n", msg))
}

func handleGitHubStop(w http.ResponseWriter, r *http.Request) {
	if ghState.Codespace.Name == "" {
		http.Error(w, "No active codespace", 400)
		return
	}
	url := fmt.Sprintf("https://api.github.com/user/codespaces/%s/stop", ghState.Codespace.Name)
	resp, err := ghAPIRequest("POST", url, cfg.GitHub.Token, nil)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer resp.Body.Close()

	ghMu.Lock()
	ghState.Codespace.Status = "stopped"
	ghState.Status = "idle"
	ghMu.Unlock()
	broadcastLog("[GitHub] Codespace stopped.\n")
	json.NewEncoder(w).Encode(map[string]string{"status": "stopped"})
}

func handleGitHubDelete(w http.ResponseWriter, r *http.Request) {
	if ghState.Codespace.Name == "" {
		http.Error(w, "No active codespace", 400)
		return
	}
	url := fmt.Sprintf("https://api.github.com/user/codespaces/%s", ghState.Codespace.Name)
	resp, err := ghAPIRequest("DELETE", url, cfg.GitHub.Token, nil)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer resp.Body.Close()

	ghMu.Lock()
	ghState.Codespace = gitHubCodespaceState{}
	ghState.Status = "idle"
	ghMu.Unlock()
	broadcastLog("[GitHub] Codespace deleted.\n")
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

func handleGitHubStatus(w http.ResponseWriter, r *http.Request) {
	ghMu.Lock()
	defer ghMu.Unlock()
	if ghState.Status == "" {
		ghState.Status = "idle"
	}
	json.NewEncoder(w).Encode(ghState)
}

// ---------- Xray management ----------
var (
	xrayCmd   *exec.Cmd
	xrayMutex sync.Mutex
)

func ensureXrayBinary() (string, error) {
	binPath := cfg.Xray.BinPath
	if binPath == "" {
		binDir := filepath.Join(config.GetDocsDir(), "xray")
		if runtime.GOOS == "windows" {
			binPath = filepath.Join(binDir, "xray.exe")
		} else {
			binPath = filepath.Join(binDir, "xray")
		}
	}

	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		broadcastLog("[Xray] Binary missing. Downloading...\n")
		os.MkdirAll(filepath.Dir(binPath), 0755)

		var url string
		switch runtime.GOOS {
		case "windows":
			url = "https://github.com/XTLS/Xray-core/releases/download/v1.8.23/Xray-windows-64.zip"
		case "linux":
			url = "https://github.com/XTLS/Xray-core/releases/download/v1.8.23/Xray-linux-64.zip"
		case "darwin":
			url = "https://github.com/XTLS/Xray-core/releases/download/v1.8.23/Xray-macos-64.zip"
		default:
			return "", fmt.Errorf("unsupported OS: %s", runtime.GOOS)
		}

		resp, err := http.Get(url)
		if err != nil {
			return "", fmt.Errorf("download failed: %v", err)
		}
		defer resp.Body.Close()

		tmpZip := filepath.Join(os.TempDir(), "xray_download.zip")
		f, err := os.Create(tmpZip)
		if err != nil {
			return "", err
		}
		io.Copy(f, resp.Body)
		f.Close()
		defer os.Remove(tmpZip)

		if runtime.GOOS == "windows" {
			cmd := exec.Command("powershell", "-Command", fmt.Sprintf("Expand-Archive -Force -Path '%s' -DestinationPath '%s'", tmpZip, filepath.Dir(binPath)))
			if err := cmd.Run(); err != nil {
				return "", fmt.Errorf("extract failed: %v", err)
			}
		} else {
			cmd := exec.Command("unzip", "-o", tmpZip, "-d", filepath.Dir(binPath))
			if err := cmd.Run(); err != nil {
				return "", fmt.Errorf("extract failed: %v", err)
			}
			os.Chmod(binPath, 0755)
		}
		broadcastLog("[Xray] Binary downloaded and extracted.\n")
	}
	return binPath, nil
}

func generateXrayConfig(vlessURL string, socksPort int) (string, error) {
	// Extract SNI from vless URL
	sni := ""
	if parts := strings.Split(vlessURL, "sni="); len(parts) > 1 {
		sni = strings.Split(parts[1], "&")[0]
	}

	xrayConfig := map[string]interface{}{
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
				"protocol": "vless",
				"settings": map[string]interface{}{
					"vnext": []map[string]interface{}{
						{
							"address": "94.130.50.12",
							"port":    443,
							"users": []map[string]interface{}{
								{
									"id":         "550e8400-e29b-41d4-a716-446655440000",
									"encryption": "none",
								},
							},
						},
					},
				},
				"streamSettings": map[string]interface{}{
					"network":  "xhttp",
					"security": "tls",
					"tlsSettings": map[string]interface{}{
						"serverName":    sni,
						"allowInsecure": false,
					},
					"xhttpSettings": map[string]interface{}{
						"mode": "packet-up",
						"path": "/",
					},
				},
			},
		},
	}

	jsonBytes, _ := json.MarshalIndent(xrayConfig, "", "  ")
	configDir := filepath.Join(config.GetDocsDir(), "xray")
	os.MkdirAll(configDir, 0755)
	configPath := filepath.Join(configDir, "config.json")
	if err := os.WriteFile(configPath, jsonBytes, 0644); err != nil {
		return "", err
	}
	return configPath, nil
}

func handleXrayStart(w http.ResponseWriter, r *http.Request) {
	xrayMutex.Lock()
	defer xrayMutex.Unlock()

	if xrayCmd != nil && (xrayCmd.Process != nil && xrayCmd.ProcessState == nil) {
		json.NewEncoder(w).Encode(map[string]string{"status": "already_running"})
		return
	}

	vless := ghState.Codespace.VLESS
	if vless == "" {
		http.Error(w, "No VLESS link available", 400)
		return
	}

	binPath, err := ensureXrayBinary()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	configPath, err := generateXrayConfig(vless, cfg.Xray.Port)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	xrayCmd = exec.Command(binPath, "run", "-config", configPath)
	if err := xrayCmd.Start(); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	broadcastLog(fmt.Sprintf("[Xray] Process started (PID: %d) on SOCKS5 port %d\n", xrayCmd.Process.Pid, cfg.Xray.Port))
	json.NewEncoder(w).Encode(map[string]string{"status": "started"})
}

func handleXrayStop(w http.ResponseWriter, r *http.Request) {
	xrayMutex.Lock()
	defer xrayMutex.Unlock()

	if xrayCmd != nil && xrayCmd.Process != nil {
		xrayCmd.Process.Kill()
		xrayCmd = nil
		broadcastLog("[Xray] Process stopped.\n")
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "stopped"})
}

func openURL(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	cmd.Start()
}
