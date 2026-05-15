// Package metrics serves a Prometheus-text-format /metrics endpoint exposing
// per-device gauges sourced from aries driver ioctls.
package metrics

import (
	"fmt"
	"io"
	"net/http"
	"path/filepath"

	"k8s.io/klog/v2"
	"mobilint-device-plugin/pkg/config"
	"mobilint-device-plugin/pkg/plugin/aries"
)

// NewHandler returns an http.Handler serving /metrics.
func NewHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", handleMetrics)
	return mux
}

type deviceSample struct {
	id     string
	sample aries.Sample
	err    error
}

func collect() []deviceSample {
	paths, err := filepath.Glob(config.DevicePattern)
	if err != nil {
		klog.Errorf("metrics: glob %s: %v", config.DevicePattern, err)
		return nil
	}
	out := make([]deviceSample, 0, len(paths))
	for _, p := range paths {
		s, err := aries.ReadSample(p)
		if err != nil {
			klog.V(2).Infof("metrics: ReadSample %s: %v", p, err)
		}
		out = append(out, deviceSample{id: filepath.Base(p), sample: s, err: err})
	}
	return out
}

func handleMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	samples := collect()

	// Health: emitted for every device regardless of error.
	fmt.Fprintln(w, "# HELP mobilint_npu_health 1 if device responds to DRIVER_INFO ioctl, 0 otherwise.")
	fmt.Fprintln(w, "# TYPE mobilint_npu_health gauge")
	for _, x := range samples {
		v := 0
		if x.err == nil {
			v = 1
		}
		fmt.Fprintf(w, "mobilint_npu_health{device=%q} %d\n", x.id, v)
	}

	emit := func(name, help string, get func(aries.Sample) int32) {
		emitMetric(w, samples, name, help, get)
	}
	emit("mobilint_npu_temperature", "Raw temperature value from ARIES_IOC_GET_TEMPERATURE.",
		func(s aries.Sample) int32 { return s.Temperature })
	emit("mobilint_npu_clock_npu_hz", "NPU clock in Hz (ARIES_IOC_GET_CLOCK_NPU).",
		func(s aries.Sample) int32 { return s.ClockNPUHz })
	emit("mobilint_npu_clock_noc_hz", "NoC clock in Hz (ARIES_IOC_GET_CLOCK_NOC).",
		func(s aries.Sample) int32 { return s.ClockNOCHz })
	emit("mobilint_npu_power_total", "Raw total power from ARIES_IOC_GET_TOTAL_POWER.",
		func(s aries.Sample) int32 { return s.PowerTotal })
	emit("mobilint_npu_current_total", "Raw total current from ARIES_IOC_GET_TOTAL_CURRENT.",
		func(s aries.Sample) int32 { return s.CurrentTotal })
	emit("mobilint_npu_voltage_total", "Raw total voltage from ARIES_IOC_GET_TOTAL_VOLTAGE.",
		func(s aries.Sample) int32 { return s.VoltageTotal })
	emit("mobilint_npu_fan_duty", "Fan duty cycle from ARIES_IOC_GET_FAN_DUTY.",
		func(s aries.Sample) int32 { return s.FanDuty })
	emit("mobilint_npu_fd_count", "Open fd count for this device from ARIES_IOC_GET_FD_COUNT.",
		func(s aries.Sample) int32 { return s.FDCount })
}

func emitMetric(w io.Writer, samples []deviceSample, name, help string, get func(aries.Sample) int32) {
	fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s gauge\n", name, help, name)
	for _, x := range samples {
		if x.err != nil {
			continue
		}
		fmt.Fprintf(w, "%s{device=%q} %d\n", name, x.id, get(x.sample))
	}
}
