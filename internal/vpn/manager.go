package vpn

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/LeoBarbash/family-vpn/internal/config"
)

// Status describes the current tunnel state exposed to the UI.
type Status struct {
	Connected bool   `json:"connected"`
	Interface string `json:"interface"`
	Endpoint  string `json:"endpoint"`
	Message   string `json:"message"`
	Since     string `json:"since,omitempty"`
}

// Manager controls an AmneziaWG userspace tunnel.
//
// Phase 1 shells out to the amneziawg-go binary. Later you can embed the
// library directly or add a privileged helper for macOS Network Extension.
type Manager struct {
	mu sync.Mutex

	profile   *config.Profile
	cmd       *exec.Cmd
	cancel    context.CancelFunc
	iface     string
	connected bool
	since     time.Time
	lastError string
}

func NewManager() *Manager {
	return &Manager{}
}

func (m *Manager) LoadProfile(name, wgQuick string) (*config.Profile, error) {
	profile, err := config.ParseWgQuick(name, wgQuick)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.connected {
		return nil, fmt.Errorf("disconnect before loading a new profile")
	}
	m.profile = profile
	m.lastError = ""
	return profile, nil
}

func (m *Manager) Connect(ctx context.Context) error {
	m.mu.Lock()
	if m.connected {
		m.mu.Unlock()
		return fmt.Errorf("already connected")
	}
	profile := m.profile
	m.mu.Unlock()

	if profile == nil {
		return fmt.Errorf("no profile loaded")
	}

	binary, err := findAmneziaWGBinary()
	if err != nil {
		return err
	}

	iface := defaultInterfaceName()
	runCtx, cancel := context.WithCancel(ctx)

	cmd := exec.CommandContext(runCtx, binary, "-f", iface)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("start amneziawg-go: %w", err)
	}

	// Give the TUN interface a moment to come up before applying config.
	time.Sleep(500 * time.Millisecond)

	confPath, cleanup, err := writeTempConfig(profile)
	if err != nil {
		cancel()
		_ = cmd.Process.Kill()
		return err
	}
	defer cleanup()

	if err := applyConfig(binary, iface, confPath); err != nil {
		cancel()
		_ = cmd.Process.Kill()
		return err
	}

	if err := configureRoutes(profile, iface); err != nil {
		cancel()
		_ = cmd.Process.Kill()
		return err
	}

	m.mu.Lock()
	m.cmd = cmd
	m.cancel = cancel
	m.iface = iface
	m.connected = true
	m.since = time.Now()
	m.lastError = ""
	m.mu.Unlock()

	go m.waitProcess(cmd, cancel)
	return nil
}

func (m *Manager) Disconnect() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.connected {
		return nil
	}

	if m.cancel != nil {
		m.cancel()
	}
	if m.cmd != nil && m.cmd.Process != nil {
		_ = m.cmd.Process.Kill()
	}

	m.connected = false
	m.cmd = nil
	m.cancel = nil
	m.iface = ""
	m.since = time.Time{}
	return nil
}

func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()

	st := Status{
		Connected: m.connected,
		Interface: m.iface,
		Message:   m.lastError,
	}
	if m.profile != nil {
		st.Endpoint = m.profile.Endpoint
	}
	if m.connected && !m.since.IsZero() {
		st.Since = m.since.UTC().Format(time.RFC3339)
	}
	if st.Message == "" && m.connected {
		st.Message = "connected"
	}
	if st.Message == "" {
		st.Message = "disconnected"
	}
	return st
}

func (m *Manager) waitProcess(cmd *exec.Cmd, cancel context.CancelFunc) {
	err := cmd.Wait()
	cancel()

	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.connected {
		return
	}
	m.connected = false
	m.cmd = nil
	m.cancel = nil
	m.iface = ""
	if err != nil {
		m.lastError = err.Error()
	} else {
		m.lastError = "tunnel stopped"
	}
}

func findAmneziaWGBinary() (string, error) {
	if path := os.Getenv("AMNEZIAWG_GO"); path != "" {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	for _, name := range []string{"amneziawg-go", "amnezia-wg-go"} {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf(`amneziawg-go not found.

Install it:
  git clone https://github.com/amnezia-vpn/amneziawg-go
  cd amneziawg-go && make && sudo cp amneziawg-go /usr/local/bin/

Or set AMNEZIAWG_GO=/path/to/amneziawg-go`)
}

func defaultInterfaceName() string {
	switch runtime.GOOS {
	case "darwin":
		return "utun0"
	case "linux":
		return "awg0"
	default:
		return "awg0"
	}
}

func writeTempConfig(profile *config.Profile) (string, func(), error) {
	dir, err := os.MkdirTemp("", "family-vpn-*")
	if err != nil {
		return "", func() {}, err
	}

	path := filepath.Join(dir, profile.Name+".conf")
	if err := os.WriteFile(path, []byte(profile.Raw), 0o600); err != nil {
		os.RemoveAll(dir)
		return "", func() {}, err
	}

	cleanup := func() { _ = os.RemoveAll(dir) }
	return path, cleanup, nil
}

func applyConfig(binary, iface, confPath string) error {
	// Prefer awg/setconf when available; fall back to wg-compatible tools.
	for _, args := range [][]string{
		{"awg", "setconf", iface, confPath},
		{"wg", "setconf", iface, confPath},
		{filepath.Base(binary), "setconf", iface, confPath},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err == nil {
			return nil
		}
	}

	return fmt.Errorf("could not apply config with awg/wg setconf; install amneziawg-tools")
}

func configureRoutes(profile *config.Profile, iface string) error {
	switch runtime.GOOS {
	case "darwin":
		// Full-tunnel routing on macOS requires a privileged helper or manual
		// route commands. Keep phase 1 minimal and surface a clear message.
		if len(profile.DNS) > 0 {
			return fmt.Errorf("macOS route/DNS setup is not automated yet; interface %q is up — add routes manually or implement a Network Extension helper", iface)
		}
		return fmt.Errorf("macOS routing helper not implemented yet for interface %q", iface)
	case "linux":
		return configureLinuxRoutes(profile, iface)
	default:
		return fmt.Errorf("unsupported platform %s", runtime.GOOS)
	}
}

func configureLinuxRoutes(profile *config.Profile, iface string) error {
	for _, addr := range profile.Address {
		ip, _, err := netParseCIDR(addr)
		if err != nil {
			return err
		}
		cmd := exec.Command("ip", "addr", "add", addr, "dev", iface)
		if out, err := cmd.CombinedOutput(); err != nil && !strings.Contains(string(out), "File exists") {
			return fmt.Errorf("ip addr add: %w (%s)", err, strings.TrimSpace(string(out)))
		}
		_ = ip
	}

	cmd := exec.Command("ip", "link", "set", "up", "dev", iface)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ip link set up: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func netParseCIDR(s string) (net.IP, *net.IPNet, error) {
	if !strings.Contains(s, "/") {
		s += "/32"
	}
	return net.ParseCIDR(s)
}
