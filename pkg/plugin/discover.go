package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"mobilint-device-plugin/pkg/config"
	"mobilint-device-plugin/pkg/plugin/aries"

	"golang.org/x/sys/unix"
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

		dev := &pluginapi.Device{ID: id, Health: health}

		numa := numaNode(path)
		if numa >= 0 { // Report NUMA topology when available
			dev.Topology = &pluginapi.TopologyInfo{
				Nodes: []*pluginapi.NUMANode{{ID: int64(numa)}},
			}
		}
		devices = append(devices, dev)

		klog.V(2).Infof("discovered device id=%s path=%s health=%s numa=%d", id, path, health, numa)
	}

	return devices
}

func numaNode(devPath string) int {
	var st unix.Stat_t
	if err := unix.Stat(devPath, &st); err != nil {
		return -1
	}
	sysDir, err := filepath.EvalSymlinks(
		fmt.Sprintf("/sys/dev/char/%d:%d", unix.Major(st.Rdev), unix.Minor(st.Rdev)))
	if err != nil {
		return -1
	}
	return numaNodeFromSysfs(sysDir)
}

func numaNodeFromSysfs(dir string) int {
	for dir != "/" && dir != "." {
		if b, err := os.ReadFile(filepath.Join(dir, "numa_node")); err == nil {
			if n, err := strconv.Atoi(strings.TrimSpace(string(b))); err == nil {
				return n
			}
		}
		dir = filepath.Dir(dir)
	}
	return -1
}
