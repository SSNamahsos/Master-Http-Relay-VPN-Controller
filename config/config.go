package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// DefaultConfig returns a Config with default values.
func DefaultConfig() Config {
	return Config{
		Mode:           "apps_script",
		GoogleIP:       "216.239.38.120",
		FrontDomain:    "www.google.com",
		ListenHost:     "127.0.0.1",
		ListenPort:     8085,
		Socks5Port:     1080,
		Socks5Enabled:  true,
		LogLevel:       "INFO",
		VerifySSL:      true,
		HealthInterval: 5,
		WorkerEnabled:  false,
		ExitNode: ExitNodeConfig{
			Mode: "selective",
		},
		UpstreamAuthKey:      "",
		LogLimitChars:        15000,
		LogLevelSetting:      "Normal",
		GitHub: GitHubConfig{
			Enabled:      false,
			ClientID:     "",
			ClientSecret: "",
			Token:        "",
			RepoOwner:    "amiremohamadi",
			RepoName:     "g2ray",
			Branch:       "main",
			Machine:      "standardLinux2x4",
		},
		Xray: XrayConfig{
			Port:    10808,
			BinPath: "", // دانلود خودکار
		},
	}
}

// GetDocsDir returns the path to Documents\\MihaniRelay (cross‑platform).
func GetDocsDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Documents", "MihaniRelay")
}

// configPathForMode returns the full path of the config file for a given mode.
func configPathForMode(mode string) string {
	return filepath.Join(GetDocsDir(), fmt.Sprintf("config_%s.json", mode))
}

// LoadMode loads the configuration for a specific mode from disk.
// If the file doesn't exist, it returns a default configuration for that mode.
func LoadMode(mode string) Config {
	path := configPathForMode(mode)
	c := DefaultConfig()
	c.Mode = mode
	data, err := os.ReadFile(path)
	if err == nil {
		_ = json.Unmarshal(data, &c)
	}
	if c.LogLevel == "" {
		c.LogLevel = "INFO"
	}
	if len(c.GoogleSNIPool) == 0 {
		c.GoogleSNIPool = defaultSNIPool()
	}
	return c
}

// SaveMode saves the configuration for the mode specified in the Config struct.
func SaveMode(c Config) {
	path := configPathForMode(c.Mode)
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	data, _ := json.MarshalIndent(c, "", " ")
	_ = os.WriteFile(path, data, 0644)
}

// Load (backward compat) loads the default mode (apps_script).
func Load() Config {
	return LoadMode("apps_script")
}

// Save (backward compat) saves using the mode in the config struct.
func Save(c Config) {
	SaveMode(c)
}

// GitHubConfig holds settings for using GitHub Codespaces (G2Ray).
type GitHubConfig struct {
	Enabled      bool   `json:"Enabled"`
	ClientID     string `json:"ClientID"`
	ClientSecret string `json:"ClientSecret"`
	Token        string `json:"Token"`
	RepoOwner    string `json:"RepoOwner"`
	RepoName     string `json:"RepoName"`
	Branch       string `json:"Branch"`
	Machine      string `json:"Machine"`
}

// XrayConfig holds settings for the local Xray core.
type XrayConfig struct {
	Port    int    `json:"Port"`    // پورت SOCKS5 ورودی (پیش‌فرض 10808)
	BinPath string `json:"BinPath"` // مسیر فایل xray.exe (خودکار دانلود در صورت خالی)
}

type Account struct {
	AuthKey   string   `json:"AuthKey"`
	ScriptIDs []string `json:"ScriptIDs"`
}

type ExitNodeConfig struct {
	Enabled  bool     `json:"Enabled"`
	RelayURL string   `json:"RelayURL"`
	PSK      string   `json:"PSK"`
	Hosts    []string `json:"Hosts"`
	Mode     string   `json:"Mode"`
}

type V2RayConfig struct {
	ID      string `json:"ID"`
	Name    string `json:"Name"`
	Type    string `json:"Type"` // shadowsocks, v2ray, npvt, etc.
	Content string `json:"Content"`
	Active  bool   `json:"Active"`
}

type Config struct {
	Mode                 string          `json:"Mode"`
	GoogleIP             string          `json:"GoogleIP"`
	FrontDomain          string          `json:"FrontDomain"`
	GoogleSNIPool        []string        `json:"GoogleSNIPool,omitempty"`
	ScriptID             string          `json:"ScriptID,omitempty"`
	ScriptIDs            []string        `json:"ScriptIDs,omitempty"`
	AuthKey              string          `json:"AuthKey"`
	ListenHost           string          `json:"ListenHost"`
	ListenPort           int             `json:"ListenPort"`
	Socks5Port           int             `json:"Socks5Port"`
	Socks5Enabled        bool            `json:"Socks5Enabled"`
	LogLevel             string          `json:"LogLevel"`
	VerifySSL            bool            `json:"VerifySSL"`
	AntiBan              bool            `json:"AntiBan"`
	ForceRelayYouTube    bool            `json:"ForceRelayYouTube"`
	LanSharing           bool            `json:"LanSharing"`
	PerformanceMode      bool            `json:"PerformanceMode"`
	HealthInterval       int             `json:"HealthInterval"`
	CustomDirectHosts    []string        `json:"CustomDirectHosts,omitempty"`
	FrontingGroups       []interface{}   `json:"FrontingGroups,omitempty"`
	ExitNode             ExitNodeConfig  `json:"ExitNode"`
	Accounts             []Account       `json:"Accounts,omitempty"`
	WorkerURL            string          `json:"WorkerURL,omitempty"`
	WorkerEnabled        bool            `json:"WorkerEnabled,omitempty"`
	UpstreamForwarderURL string          `json:"UpstreamForwarderURL,omitempty"`
	UpstreamAuthKey      string          `json:"UpstreamAuthKey,omitempty"`
	LogLimitChars        int             `json:"LogLimitChars"`
	LogLevelSetting      string          `json:"LogLevelSetting"` // Low, Normal, High, Unlimited
	GitHub               GitHubConfig    `json:"GitHub,omitempty"`
	Xray                 XrayConfig      `json:"Xray,omitempty"`
	V2RayConfigs         []V2RayConfig   `json:"V2RayConfigs,omitempty"`
}

func defaultSNIPool() []string {
	return []string{
		"www.google.com", "mail.google.com", "accounts.google.com",
		"www.googleapis.com", "googleapis.com", "www.gstatic.com",
		"fonts.googleapis.com", "ajax.googleapis.com", "maps.googleapis.com",
		"lh3.googleusercontent.com", "lh4.googleusercontent.com",
		"lh5.googleusercontent.com", "lh6.googleusercontent.com",
		"www.google-analytics.com", "ssl.google-analytics.com",
		"www.googletagmanager.com", "googletagmanager.com",
		"www.youtube.com", "m.youtube.com", "i.ytimg.com", "s.ytimg.com",
		"yt3.ggpht.com", "play.google.com", "drive.google.com",
		"docs.google.com", "sheets.google.com", "slides.google.com",
		"calendar.google.com", "meet.google.com", "photos.google.com",
	}
}