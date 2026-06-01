package plugin

import (
	"context"
	"fmt"

	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
	"mobilint-device-plugin/pkg/config"
)

func (p *MobilintDevicePlugin) Allocate(
	ctx context.Context,
	reqs *pluginapi.AllocateRequest,
) (*pluginapi.AllocateResponse, error) {
	resp := &pluginapi.AllocateResponse{}

	for _, req := range reqs.ContainerRequests {
		containerResp := &pluginapi.ContainerAllocateResponse{}

		for _, id := range req.DevicesIds {
			if !p.isHealthy(id) {
				return nil, fmt.Errorf("device %s is not healthy", id)
			}

			containerResp.CdiDevices = append(containerResp.CdiDevices, &pluginapi.CDIDevice{
				Name: config.ResourceName + "=" + id,
			})
		}

		resp.ContainerResponses = append(resp.ContainerResponses, containerResp)
	}

	return resp, nil
}
