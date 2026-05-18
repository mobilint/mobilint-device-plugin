package plugin

import (
	"context"
	"sync"
	"sync/atomic"

	"google.golang.org/grpc"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

type MobilintDevicePlugin struct {
	pluginapi.UnimplementedDevicePluginServer

	socket    string
	server    *grpc.Server
	serverErr chan error

	mu      sync.RWMutex
	devices []*pluginapi.Device

	healthCh      chan struct{}
	monitorCancel context.CancelFunc

	registered atomic.Bool
}

func New(socket string) *MobilintDevicePlugin {
	return &MobilintDevicePlugin{
		socket:    socket,
		serverErr: make(chan error, 1),
		healthCh:  make(chan struct{}, 1),
	}
}

func (p *MobilintDevicePlugin) SetRegistered(v bool) {
	p.registered.Store(v)
}

func (p *MobilintDevicePlugin) IsRegistered() bool {
	return p.registered.Load()
}

func (p *MobilintDevicePlugin) setDevices(devices []*pluginapi.Device) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.devices = devices
}

func (p *MobilintDevicePlugin) isHealthy(id string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for _, d := range p.devices {
		if d.ID == id && d.Health == pluginapi.Healthy {
			return true
		}
	}
	return false
}
