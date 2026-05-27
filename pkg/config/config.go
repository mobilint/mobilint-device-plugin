package config

const (
	ResourceName      = "mobilint.com/npu"
	DevicePattern     = "/dev/aries[0-9]*"
	PluginSocketName  = "mobilint.sock"
	VisibleDevicesEnv = "MOBILINT_VISIBLE_DEVICES"

	DiscoveryIntervalSeconds = 5
	RegisterTimeoutSeconds   = 5

	// Exponential backoff for kubelet registration retries.
	RegisterBackoffInitialSeconds = 1
	RegisterBackoffMaxSeconds     = 60

	// Escalate repeated registration failures from warning to error logs.
	RegisterFailureLogThreshold = 10

	MetricsAddr = ":9400"

	MetricsReadHeaderTimeoutSeconds = 5
	MetricsReadTimeoutSeconds       = 10
	MetricsWriteTimeoutSeconds      = 10
	MetricsIdleTimeoutSeconds       = 30
)
