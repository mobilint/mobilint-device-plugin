package plugin

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
	"mobilint-device-plugin/pkg/config"
	"tags.cncf.io/container-device-interface/specs-go"
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
	// Pure CDI: device injection is delegated to the CDI spec, so the response
	// carries only CDI device names — no legacy Devices and no env.
	if len(cr.Devices) != 0 {
		t.Errorf("Devices=%d, want 0 (CDI handles injection)", len(cr.Devices))
	}
	if len(cr.Envs) != 0 {
		t.Errorf("Envs=%v, want none", cr.Envs)
	}
	want := []string{"mobilint.com/npu=aries0", "mobilint.com/npu=aries1"}
	if len(cr.CdiDevices) != len(want) {
		t.Fatalf("CdiDevices=%d, want %d", len(cr.CdiDevices), len(want))
	}
	for i, w := range want {
		if got := cr.CdiDevices[i].Name; got != w {
			t.Errorf("CdiDevices[%d]=%q, want %q", i, got, w)
		}
	}
}

func TestWriteCDISpec(t *testing.T) {
	dir := t.TempDir()
	devices := []*pluginapi.Device{
		{ID: "aries0", Health: pluginapi.Healthy},
		{ID: "aries1", Health: pluginapi.Healthy},
	}

	if err := writeCDISpec(dir, devices); err != nil {
		t.Fatalf("writeCDISpec: %v", err)
	}

	matches, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	if len(matches) != 1 {
		t.Fatalf("expected 1 spec file, found %v", matches)
	}

	data, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("read spec: %v", err)
	}
	var spec specs.Spec
	if err := json.Unmarshal(data, &spec); err != nil {
		t.Fatalf("unmarshal spec: %v", err)
	}

	if spec.Kind != config.ResourceName {
		t.Errorf("kind=%q, want %q", spec.Kind, config.ResourceName)
	}
	if spec.Version == "" {
		t.Error("cdiVersion is empty")
	}
	if len(spec.Devices) != 2 {
		t.Fatalf("devices=%d, want 2", len(spec.Devices))
	}
	for i, want := range []string{"aries0", "aries1"} {
		d := spec.Devices[i]
		if d.Name != want {
			t.Errorf("devices[%d].name=%q, want %q", i, d.Name, want)
		}
		nodes := d.ContainerEdits.DeviceNodes
		if len(nodes) != 1 || nodes[0].Path != "/dev/"+want || nodes[0].Permissions != "rw" {
			t.Errorf("devices[%d] deviceNodes=%+v, want /dev/%s rw", i, nodes, want)
		}
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
