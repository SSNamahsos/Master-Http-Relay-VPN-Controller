package cert

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"mhrv-go/config"
	"mhrv-go/mitm"
)

func caDir() string {
	return filepath.Join(config.GetDocsDir(), "ca")
}

func caCertPath() string {
	return filepath.Join(caDir(), "ca.crt")
}

func EnsureCAFiles() {
	dir := caDir()
	os.MkdirAll(dir, 0700)
	_ = mitm.NewCertManager()
}

func EnsureCA() error {
	os.MkdirAll(caDir(), 0700)
	return nil
}

func IsCAInstalled() bool {
	switch runtime.GOOS {
	case "windows":
		return isCAInstalledWindows()
	case "darwin":
		return isCAInstalledMacOS()
	case "linux":
		return isCAInstalledLinux()
	default:
		return false
	}
}

func InstallCACert() error {
	EnsureCAFiles()

	switch runtime.GOOS {
	case "windows":
		return installCAWindows()
	case "darwin":
		return installCAMacOS()
	case "linux":
		return installCALinux()
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

// ─── Windows ─────────────────────────────────

func isCAInstalledWindows() bool {
	cmd := exec.Command("certutil", "-user", "-store", "Root", "MihaniRelay MITM CA")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Run() == nil
}

func installCAWindows() error {
	cmd := exec.Command("certutil", "-addstore", "-user", "Root", caCertPath())
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Run()
}

// ─── macOS ───────────────────────────────────

func isCAInstalledMacOS() bool {
	cmd := exec.Command("security", "find-certificate", "-c", "MihaniRelay MITM CA")
	// macOS doesn't have HideWindow, but it's harmless to set zero value
	return cmd.Run() == nil
}

func installCAMacOS() error {
	cmd := exec.Command("security", "add-trusted-cert", "-d", "-r", "trustRoot",
		"-k", os.ExpandEnv("$HOME/Library/Keychains/login.keychain-db"), caCertPath())
	return cmd.Run()
}

// ─── Linux ───────────────────────────────────

func isCAInstalledLinux() bool {
	data, err := os.ReadFile(caCertPath())
	if err != nil {
		return false
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return false
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return false
	}
	destPath := fmt.Sprintf("/usr/local/share/ca-certificates/mihani-relay-%s.crt",
		strings.ReplaceAll(cert.Subject.CommonName, " ", "_"))
	_, err = os.Stat(destPath)
	return err == nil
}

func installCALinux() error {
	data, err := os.ReadFile(caCertPath())
	if err != nil {
		return err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return fmt.Errorf("invalid CA certificate")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return err
	}
	destPath := fmt.Sprintf("/usr/local/share/ca-certificates/mihani-relay-%s.crt",
		strings.ReplaceAll(cert.Subject.CommonName, " ", "_"))

	if err := os.WriteFile(destPath, data, 0644); err != nil {
		cmd := exec.Command("sudo", "cp", caCertPath(), destPath)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to copy CA certificate: %v", err)
		}
	}

	updateCmd := exec.Command("update-ca-certificates")
	if err := updateCmd.Run(); err != nil {
		cmd := exec.Command("sudo", "update-ca-certificates")
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	return nil
}