package plugin

import (
	"context"
	"strings"
	"testing"

	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
	"mobilint-device-plugin/pkg/config"
)

func newWithDevices(devices []*pluginapi.Device) *MobilintDevicePlugin {
	p := New("/tmp/test.sock")
	p.setDevices(devices)
	return p
}

func TestIsHealthy(t *testing.T) {
	p := newWithDevices([]*pluginapi.Device{
		{ID: "aries0", Health: pluginapi.Healthy},
		{ID: "aries1", Health: pluginapi.Unhealthy},
	})

	cases := []struct {
		id   string
		want bool
	}{
		{"aries0", true},
		{"aries1", false},
		{"aries9", false},
	}
	for _, c := range cases {
		if got := p.isHealthy(c.id); got != c.want {
			t.Errorf("isHealthy(%q)=%v, want %v", c.id, got, c.want)
		}
	}
}

func TestAllocateRejectsUnhealthy(t *testing.T) {
	p := newWithDevices([]*pluginapi.Device{
		{ID: "aries0", Health: pluginapi.Healthy},
		{ID: "aries1", Health: pluginapi.Unhealthy},
	})

	_, err := p.Allocate(context.Background(), &pluginapi.AllocateRequest{
		ContainerRequests: []*pluginapi.ContainerAllocateRequest{
			{DevicesIds: []string{"aries0", "aries1"}},
		},
	})
	if err == nil {
		t.Fatal("expected error for unhealthy device, got nil")
	}
}

func TestAllocateHealthyResponse(t *testing.T) {
	p := newWithDevices([]*pluginapi.Device{
		{ID: "aries0", Health: pluginapi.Healthy},
		{ID: "aries1", Health: pluginapi.Healthy},
	})

	resp, err := p.Allocate(context.Background(), &pluginapi.AllocateRequest{
		ContainerRequests: []*pluginapi.ContainerAllocateRequest{
			{DevicesIds: []string{"aries0", "aries1"}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.ContainerResponses) != 1 {
		t.Fatalf("ContainerResponses=%d, want 1", len(resp.ContainerResponses))
	}
	cr := resp.ContainerResponses[0]
	if len(cr.Devices) != 2 {
		t.Errorf("Devices=%d, want 2", len(cr.Devices))
	}
	for _, d := range cr.Devices {
		if d.HostPath != d.ContainerPath {
			t.Errorf("HostPath %q != ContainerPath %q", d.HostPath, d.ContainerPath)
		}
		if !strings.HasPrefix(d.HostPath, "/dev/") {
			t.Errorf("HostPath %q missing /dev/ prefix", d.HostPath)
		}
		if d.Permissions != "rw" {
			t.Errorf("Permissions=%q, want rw", d.Permissions)
		}
	}
	if got := cr.Envs[config.VisibleDevicesEnv]; got != "aries0,aries1" {
		t.Errorf("%s=%q, want aries0,aries1", config.VisibleDevicesEnv, got)
	}
}

func TestDeviceSignatureChangeDetection(t *testing.T) {
	a := []*pluginapi.Device{
		{ID: "aries0", Health: pluginapi.Healthy},
		{ID: "aries1", Health: pluginapi.Healthy},
	}
	b := []*pluginapi.Device{
		{ID: "aries1", Health: pluginapi.Healthy},
		{ID: "aries0", Health: pluginapi.Healthy},
	}
	if deviceSignature(a) != deviceSignature(b) {
		t.Errorf("signature should be order-independent: %q vs %q",
			deviceSignature(a), deviceSignature(b))
	}

	c := []*pluginapi.Device{
		{ID: "aries0", Health: pluginapi.Healthy},
		{ID: "aries1", Health: pluginapi.Unhealthy},
	}
	if deviceSignature(a) == deviceSignature(c) {
		t.Errorf("signature should change on health change")
	}

	d := []*pluginapi.Device{
		{ID: "aries0", Health: pluginapi.Healthy},
	}
	if deviceSignature(a) == deviceSignature(d) {
		t.Errorf("signature should change on device set change")
	}
}
