package plugin

import (
	"fmt"

	"k8s.io/klog/v2"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
	"tags.cncf.io/container-device-interface/pkg/cdi"
	"tags.cncf.io/container-device-interface/specs-go"

	"mobilint-device-plugin/pkg/config"
)

func writeCDISpec(dir string, devices []*pluginapi.Device) error {
	cdiDevices := make([]specs.Device, 0, len(devices))
	for _, d := range devices {
		cdiDevices = append(cdiDevices, specs.Device{
			Name: d.ID,
			ContainerEdits: specs.ContainerEdits{
				DeviceNodes: []*specs.DeviceNode{
					{Path: "/dev/" + d.ID, Permissions: "rw"},
				},
			},
		})
	}

	spec := &specs.Spec{
		Kind:    config.ResourceName,
		Devices: cdiDevices,
	}

	version, err := specs.MinimumRequiredVersion(spec)
	if err != nil {
		return fmt.Errorf("determine CDI spec version: %w", err)
	}
	spec.Version = version

	cache, err := cdi.NewCache(cdi.WithSpecDirs(dir), cdi.WithAutoRefresh(false))
	if err != nil {
		return fmt.Errorf("init CDI cache: %w", err)
	}

	name := cdi.GenerateSpecName(config.CDIVendor, config.CDIClass) + ".json"
	if err := cache.WriteSpec(spec, name); err != nil {
		return fmt.Errorf("write CDI spec %s/%s: %w", dir, name, err)
	}

	klog.V(2).Infof("wrote CDI spec %s/%s for %d devices", dir, name, len(cdiDevices))
	return nil
}
