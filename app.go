package main

import (
	"context"
	"fmt"
	"os"

	"github.com/LeoBarbash/family-vpn/internal/config"
	"github.com/LeoBarbash/family-vpn/internal/vpn"
)

// App exposes VPN operations to the Wails frontend.
type App struct {
	ctx context.Context
	vpn *vpn.Manager
}

func NewApp() *App {
	return &App{vpn: vpn.NewManager()}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) LoadConfig(name, wgQuick string) (map[string]any, error) {
	profile, err := a.vpn.LoadProfile(name, wgQuick)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"name":     profile.Name,
		"address":  profile.Address,
		"dns":      profile.DNS,
		"endpoint": profile.Endpoint,
	}, nil
}

func (a *App) Connect() (vpn.Status, error) {
	if a.ctx == nil {
		a.ctx = context.Background()
	}
	if err := a.vpn.Connect(a.ctx); err != nil {
		return a.vpn.Status(), err
	}
	return a.vpn.Status(), nil
}

func (a *App) Disconnect() (vpn.Status, error) {
	if err := a.vpn.Disconnect(); err != nil {
		return a.vpn.Status(), err
	}
	return a.vpn.Status(), nil
}

func (a *App) Status() vpn.Status {
	return a.vpn.Status()
}

func (a *App) LoadConfigFromFile(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	name := config.NameFromPath(path)
	return a.LoadConfig(name, string(data))
}

func (a *App) ExampleConfig() string {
	return exampleConfig
}

const exampleConfig = `[Interface]
PrivateKey = REPLACE_WITH_CLIENT_PRIVATE_KEY
Address = 10.8.1.2/32
DNS = 1.1.1.1, 8.8.8.8
Jc = 4
Jmin = 64
Jmax = 1024
S1 = 0
S2 = 0
S3 = 0
S4 = 0

[Peer]
PublicKey = REPLACE_WITH_SERVER_PUBLIC_KEY
PresharedKey = REPLACE_WITH_PRESHARED_KEY
Endpoint = YOUR_VPS_IP:51820
AllowedIPs = 0.0.0.0/0, ::/0
PersistentKeepalive = 25
`
