package plugin

import (
	"log"
	"os"
	"path/filepath"

	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
	"mobilint-device-plugin/pkg/config"
)

func DiscoverDevices() []*pluginapi.Device {
	paths, err := filepath.Glob(config.DevicePattern)
	if err != nil {
		log.Printf("failed to glob devices: %v", err)
		return nil
	}

	devices := make([]*pluginapi.Device, 0, len(paths))

	for _, path := range paths {
		id := filepath.Base(path)
		health := pluginapi.Healthy

		if !isUsableDevice(path) {
			health = pluginapi.Unhealthy
		}

		devices = append(devices, &pluginapi.Device{
			ID:     id,
			Health: health,
		})

		log.Printf("discovered device id=%s path=%s health=%s", id, path, health)
	}

	return devices
}

// TODO: O_RDWR open만으로는 NPU의 실제 동작 가능 여부를 검증할 수 없다.
//   현재는 디바이스 노드 권한 확인 수준이며, kubelet에는 Healthy로 보고되지만
//   실제 inference 시 실패할 수 있다 (펌웨어 hang, ECC 누적, thermal throttle,
//   PCIe link degrade 등 미감지).
// Recommendation: Mobilint SDK(libmobilint/mblnctl)가 준비되면 GetDeviceStatus
//   계열 호출로 교체. SDK 미가용 시 차선책으로 raw ioctl(MOBILINT_IOC_GET_STATUS)
//   또는 sysfs(/sys/class/mobilint/<id>/health) 기반 판정으로 보강.
//   NVIDIA(NVML)/AMD(ROCm SMI)/Intel(HLML) device plugin 모두 동일 패턴.
func isUsableDevice(path string) bool {
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		log.Printf("device is not usable path=%s err=%v", path, err)
		return false
	}

	_ = file.Close()
	return true
}
