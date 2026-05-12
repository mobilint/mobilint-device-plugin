package plugin

import (
	"path/filepath"

	"k8s.io/klog/v2"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
	"mobilint-device-plugin/pkg/config"
	"mobilint-device-plugin/pkg/plugin/aries"
)

func DiscoverDevices() []*pluginapi.Device {
	paths, err := filepath.Glob(config.DevicePattern)
	if err != nil {
		klog.Errorf("failed to glob devices: %v", err)
		return nil
	}

	devices := make([]*pluginapi.Device, 0, len(paths))

	for _, path := range paths {
		id := filepath.Base(path)
		health := pluginapi.Healthy

		if err := aries.IsHealthy(path); err != nil {
			klog.Warningf("device is not usable path=%s err=%v", path, err)
			health = pluginapi.Unhealthy
		}

		devices = append(devices, &pluginapi.Device{
			ID:     id,
			Health: health,
		})

		klog.Infof("discovered device id=%s path=%s health=%s", id, path, health)
	}

	return devices
}
