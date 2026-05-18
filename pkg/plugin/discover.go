package plugin

import (
	"path/filepath"
	"regexp"
	"sort"

	"mobilint-device-plugin/pkg/config"
	"mobilint-device-plugin/pkg/plugin/aries"

	"k8s.io/klog/v2"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

var ariesIDRe = regexp.MustCompile(`^aries[0-9]+$`)

func DiscoverDevices() []*pluginapi.Device {
	paths, err := filepath.Glob(config.DevicePattern)
	if err != nil {
		klog.Errorf("failed to glob devices: %v", err)
		return nil
	}
	sort.Strings(paths)

	devices := make([]*pluginapi.Device, 0, len(paths))

	for _, path := range paths {
		id := filepath.Base(path)
		if !ariesIDRe.MatchString(id) {
			klog.Warningf("skip unexpected aries device path=%s id=%s", path, id)
			continue
		}

		health := pluginapi.Healthy
		if err := aries.IsHealthy(path); err != nil {
			klog.Warningf("device is not usable path=%s err=%v", path, err)
			health = pluginapi.Unhealthy
		}

		devices = append(devices, &pluginapi.Device{
			ID:     id,
			Health: health,
		})

		klog.V(2).Infof("discovered device id=%s path=%s health=%s", id, path, health)
	}

	return devices
}
