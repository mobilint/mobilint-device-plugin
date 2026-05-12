package plugin

import (
	"context"
	"net"
	"os"
	"sort"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"k8s.io/klog/v2"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
	"mobilint-device-plugin/pkg/config"
)

const gracefulStopTimeout = 10 * time.Second

func (p *MobilintDevicePlugin) Start() error {
	err := os.Remove(p.socket)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	listener, err := net.Listen("unix", p.socket)
	if err != nil {
		return err
	}

	p.setDevices(DiscoverDevices())

	p.server = grpc.NewServer()
	pluginapi.RegisterDevicePluginServer(p.server, p)

	go func() {
		err := p.server.Serve(listener)
		p.serverErr <- err
	}()

	ctx, cancel := context.WithCancel(context.Background())
	p.monitorCancel = cancel
	go p.runHealthMonitor(ctx)

	klog.Infof("device plugin server started socket=%s", p.socket)
	return nil
}

func (p *MobilintDevicePlugin) Err() <-chan error {
	return p.serverErr
}

func (p *MobilintDevicePlugin) Stop() {
	if p.monitorCancel != nil {
		p.monitorCancel()
	}

	if p.server != nil {
		done := make(chan struct{})

		go func() {
			p.server.GracefulStop()
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(gracefulStopTimeout):
			klog.Warningf("graceful stop timed out after %s, forcing stop", gracefulStopTimeout)
			p.server.Stop()
		}
	}

	err := os.Remove(p.socket)
	if err != nil && !os.IsNotExist(err) {
		klog.Errorf("failed to remove plugin socket %s: %v", p.socket, err)
	}
}

func (p *MobilintDevicePlugin) Register(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, time.Duration(config.RegisterTimeoutSeconds)*time.Second)
	defer cancel()

	conn, err := grpc.NewClient(
		"passthrough:"+pluginapi.KubeletSocket,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", addr)
		}),
	)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := pluginapi.NewRegistrationClient(conn)

	req := &pluginapi.RegisterRequest{
		Version:      pluginapi.Version,
		Endpoint:     config.PluginSocketName,
		ResourceName: config.ResourceName,
	}

	_, err = client.Register(ctx, req)
	return err
}

func (p *MobilintDevicePlugin) GetDevicePluginOptions(
	ctx context.Context,
	e *pluginapi.Empty,
) (*pluginapi.DevicePluginOptions, error) {
	return &pluginapi.DevicePluginOptions{}, nil
}

func (p *MobilintDevicePlugin) ListAndWatch(
	e *pluginapi.Empty,
	stream pluginapi.DevicePlugin_ListAndWatchServer,
) error {
	if err := p.sendDevices(stream); err != nil {
		return err
	}

	for {
		select {
		case <-stream.Context().Done():
			return stream.Context().Err()
		case <-p.healthCh:
			if err := p.sendDevices(stream); err != nil {
				return err
			}
		}
	}
}

func (p *MobilintDevicePlugin) PreStartContainer(
	ctx context.Context,
	req *pluginapi.PreStartContainerRequest,
) (*pluginapi.PreStartContainerResponse, error) {
	return &pluginapi.PreStartContainerResponse{}, nil
}

func (p *MobilintDevicePlugin) runHealthMonitor(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(config.DiscoveryIntervalSeconds) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.refreshDevices()
		}
	}
}

func (p *MobilintDevicePlugin) refreshDevices() {
	devices := DiscoverDevices()
	newSig := deviceSignature(devices)

	p.mu.Lock()
	oldSig := deviceSignature(p.devices)
	p.devices = devices
	p.mu.Unlock()

	if oldSig != newSig {
		select {
		case p.healthCh <- struct{}{}:
		default:
		}
	}
}

func (p *MobilintDevicePlugin) sendDevices(stream pluginapi.DevicePlugin_ListAndWatchServer) error {
	p.mu.RLock()
	snapshot := make([]*pluginapi.Device, len(p.devices))
	copy(snapshot, p.devices)
	p.mu.RUnlock()

	return stream.Send(&pluginapi.ListAndWatchResponse{Devices: snapshot})
}

func deviceSignature(devices []*pluginapi.Device) string {
	keys := make([]string, len(devices))
	for i, d := range devices {
		keys[i] = d.ID + "=" + d.Health
	}
	sort.Strings(keys)
	return strings.Join(keys, ",")
}
