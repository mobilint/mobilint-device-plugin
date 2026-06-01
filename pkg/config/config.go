package config

const (
	CDIVendor    = "mobilint.com"
	CDIClass     = "npu"
	ResourceName = CDIVendor + "/" + CDIClass

	// Host directory for dynamically generated CDI specs. /var/run/cdi is the
	// CDI standard location for transient/generated specs across runtimes.
	CDISpecDir = "/var/run/cdi"

	DevicePattern    = "/dev/aries[0-9]*"
	PluginSocketName = "mobilint.sock"

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
