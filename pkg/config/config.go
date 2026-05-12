package config

const (
	ResourceName      = "mobilint.com/npu"
	DevicePattern     = "/dev/aries[0-9]*"
	PluginSocketName  = "mobilint.sock"
	VisibleDevicesEnv = "MOBILINT_VISIBLE_DEVICES"

	DiscoveryIntervalSeconds = 5
	RegisterTimeoutSeconds   = 5
	RegisterRetrySeconds     = 30
	RegisterMaxAttempts      = 5
)
