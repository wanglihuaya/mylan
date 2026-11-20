package main

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

//go:embed embed/lanmac
var lanmacScript []byte

// report is the JSON lanmac -j prints.
type report struct {
	Version  string   `json:"version"`
	Scanned  scanned  `json:"scanned"`
	Warnings []string `json:"warnings"`
	Devices  []device `json:"devices"`
}

type scanned struct {
	Subnet     string  `json:"subnet"`
	Iface      string  `json:"iface"`
	LocalIP    string  `json:"local_ip"`
	LocalName  string  `json:"local_name"`
	Gateway    *string `json:"gateway"`
	Duration   float64 `json:"duration_s"`
	AliveHosts int     `json:"alive_hosts"`
	Bonjour    int     `json:"bonjour_devices"`
	Probed     bool    `json:"probed"`
	OUISource  string  `json:"oui_source"`
}

type device struct {
	Name         *string         `json:"name"`
	NameSource   *string         `json:"name_source"`
	Type         string          `json:"type"`
	TypeLabel    string          `json:"type_label"`
	Model        *string         `json:"model"`
	ModelName    *string         `json:"model_name"`
	IP           *string         `json:"ip"`
	Hostname     *string         `json:"hostname"`
	MAC          *string         `json:"mac"`
	Vendor       string          `json:"vendor"`
	Services     []string        `json:"services"`
	ServiceTypes []string        `json:"service_types"`
	Ports        map[string]bool `json:"ports"`
	OpenPorts    []int           `json:"open_ports"`
	Online       bool            `json:"online"`
	ARPCached    bool            `json:"arp_cached"`
	IsLocal      bool            `json:"is_local"`
	IsGateway    bool            `json:"is_gateway"`
}

// scanArgs is what lanmac receives after the script path. Filtering stays
// in the app, so a change of view does not scan again.
func scanArgs(cfg config) []string {
	args := []string{"-j"}
	if cfg.Fast {
		args = append(args, "--fast")
	}
	if cfg.Stale {
		args = append(args, "--stale")
	}
	if cfg.Names {
		args = append(args, "--names")
	}
	if s := strings.TrimSpace(cfg.Subnet); s != "" {
		args = append(args, "-s", s)
	}
	return args
}

func ensureScript(dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "lanmac")
	existing, err := os.ReadFile(path)
	if err == nil && bytes.Equal(existing, lanmacScript) {
		return path, nil
	}
	if err := os.WriteFile(path, lanmacScript, 0o755); err != nil {
		return "", err
	}
	return path, nil
}

// scanUnavailable reports that this system cannot run the macOS scanner.
// Windows would otherwise show the Microsoft Store Python alias, and the
// panel would add the macOS local-network instructions.
func scanUnavailable(goos string) error {
	switch goos {
	case "darwin", "windows":
		return nil
	case "linux":
		return fmt.Errorf("当前版本的扫描只支持 macOS，Linux 上还不能发现局域网设备")
	default:
		return fmt.Errorf("当前版本的扫描只支持 macOS")
	}
}

func pythonBin() string {
	if p, err := exec.LookPath("python3"); err == nil {
		return p
	}
	return "/usr/bin/python3"
}

// runScan executes the embedded lanmac on macOS, and the native scanner
// on Windows. A full scan pings the subnet, so it is capped at two minutes.
func runScan(dir string, cfg config) (report, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if rep, ok, err := scanNative(ctx, cfg); ok {
		return rep, err
	}
	script, err := ensureScript(dir)
	if err != nil {
		return report{}, fmt.Errorf("无法准备扫描脚本: %w", err)
	}
	if err := scanUnavailable(runtime.GOOS); err != nil {
		return report{}, err
	}
	cmd := exec.CommandContext(ctx, pythonBin(), append([]string{script}, scanArgs(cfg)...)...)
	stopScan(cmd)
	cmd.WaitDelay = 2 * time.Second
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return report{}, fmt.Errorf("扫描超时")
		}
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return report{}, fmt.Errorf("%s", firstLine(msg))
	}
	rep, err := parseReport(stdout.Bytes())
	if err != nil {
		return report{}, err
	}
	return rep, nil
}

func parseReport(b []byte) (report, error) {
	b = bytes.TrimSpace(b)
	var rep report
	if err := json.Unmarshal(b, &rep); err == nil {
		return rep, nil
	}
	i := bytes.IndexByte(b, '{')
	j := bytes.LastIndexByte(b, '}')
	if i >= 0 && j > i {
		if err := json.Unmarshal(b[i:j+1], &rep); err == nil {
			return rep, nil
		}
	}
	return report{}, fmt.Errorf("无法解析扫描结果")
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}
