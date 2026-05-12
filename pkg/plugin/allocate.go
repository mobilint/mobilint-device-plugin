package plugin

import (
	"context"
	"fmt"
	"strings"

	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
	"mobilint-device-plugin/pkg/config"
)

func (p *MobilintDevicePlugin) Allocate(
	ctx context.Context,
	reqs *pluginapi.AllocateRequest,
) (*pluginapi.AllocateResponse, error) {
	resp := &pluginapi.AllocateResponse{}

	for _, req := range reqs.ContainerRequests {
		containerResp := &pluginapi.ContainerAllocateResponse{
			Envs: map[string]string{},
		}

		visible := make([]string, 0, len(req.DevicesIds))

		for _, id := range req.DevicesIds {
			if !p.isHealthy(id) {
				return nil, fmt.Errorf("device %s is not healthy", id)
			}

			devPath := "/dev/" + id

			containerResp.Devices = append(containerResp.Devices, &pluginapi.DeviceSpec{
				HostPath:      devPath,
				ContainerPath: devPath,
				Permissions:   "rw",
			})

			visible = append(visible, id)
		}

		containerResp.Envs[config.VisibleDevicesEnv] = strings.Join(visible, ",")
		resp.ContainerResponses = append(resp.ContainerResponses, containerResp)
	}

	return resp, nil
}
