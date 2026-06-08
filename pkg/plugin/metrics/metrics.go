package metrics

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"

	"k8s.io/klog/v2"
	"mobilint-device-plugin/pkg/config"
	"mobilint-device-plugin/pkg/plugin/aries"
)

func NewHandler(ready func() bool) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", handleMetrics)
	mux.HandleFunc("/process", handleProcesses)
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if ready != nil && ready() {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	return mux
}

type deviceReading struct {
	id      string
	reading aries.Reading
	err     error
}

func collect() []deviceReading {
	paths, err := filepath.Glob(config.DevicePattern)
	if err != nil {
		klog.Errorf("metrics: glob %s: %v", config.DevicePattern, err)
		return nil
	}
	out := make([]deviceReading, 0, len(paths))
	for _, p := range paths {
		s, err := aries.Read(p)
		if err != nil {
			klog.V(2).Infof("metrics: Read %s: %v", p, err)
		}
		out = append(out, deviceReading{id: filepath.Base(p), reading: s, err: err})
	}
	return out
}

func handleMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	readings := collect()

	// Health: emitted for every device regardless of error.
	fmt.Fprintln(w, "# HELP mobilint_npu_health 1 if device monitor sample can be read, 0 otherwise.")
	fmt.Fprintln(w, "# TYPE mobilint_npu_health gauge")
	for _, x := range readings {
		v := 0
		if x.err == nil {
			v = 1
		}
		fmt.Fprintf(w, "mobilint_npu_health{device=%q} %d\n", x.id, v)
	}

	emitInfoMetric(w, readings)

	emitMetric(w, readings, "mobilint_npu_temperature_celsius", "Die temperature in degrees Celsius.",
		func(s aries.Reading) int32 { return s.Temperature })
	emitMetric(w, readings, "mobilint_npu_clock_npu_hz", "NPU clock in Hz.",
		func(s aries.Reading) int32 { return s.ClockNPUHz })
	emitMetric(w, readings, "mobilint_npu_clock_noc_hz", "NoC clock in Hz.",
		func(s aries.Reading) int32 { return s.ClockNOCHz })
	emitMetric(w, readings, "mobilint_npu_power_watts", "Total power in watts.",
		func(s aries.Reading) float64 { return s.PowerTotal / 1000 })
	emitMetric(w, readings, "mobilint_npu_current_amperes", "Total current in amperes.",
		func(s aries.Reading) float64 { return s.CurrentTotal / 1000 })
	emitMetric(w, readings, "mobilint_npu_voltage_volts", "Total supply voltage in volts.",
		func(s aries.Reading) float64 { return s.VoltageTotal / 1000 })
	emitMetric(w, readings, "mobilint_npu_fan_duty", "Fan duty cycle from the ARIES monitor.",
		func(s aries.Reading) int32 { return s.FanDuty })
	emitMetric(w, readings, "mobilint_npu_fd_count", "Open fd count for this device from ARIES_IOC_GET_FD_COUNT.",
		func(s aries.Reading) int32 { return s.FDCount })
	emitMetric(w, readings, "mobilint_npu_memory_total_bytes", "Total NPU memory in bytes.",
		func(s aries.Reading) int64 { return s.Info.MemoryTotalBytes })
	emitMetric(w, readings, "mobilint_npu_memory_used_bytes", "Used NPU memory in bytes.",
		func(s aries.Reading) uint64 { return s.MemoryUsedBytes() })
	emitMetric(w, readings, "mobilint_npu_process_count", "Number of processes currently using the NPU.",
		func(s aries.Reading) int64 { return int64(len(s.Processes)) })
	emitUtilization(w, readings)
	emitCoreMetrics(w, readings)
}

func emitUtilization(w io.Writer, readings []deviceReading) {
	fmt.Fprintln(w, "# HELP mobilint_npu_utilization_ratio Total NPU utilization, 0-1.")
	fmt.Fprintln(w, "# TYPE mobilint_npu_utilization_ratio gauge")
	for _, x := range readings {
		if x.err != nil || x.reading.UtilizationRatio < 0 {
			continue
		}
		fmt.Fprintf(w, "mobilint_npu_utilization_ratio{device=%q} %g\n", x.id, x.reading.UtilizationRatio)
	}
}

func emitCoreMetrics(w io.Writer, readings []deviceReading) {
	fmt.Fprintln(w, "# HELP mobilint_npu_core_utilization_ratio Per-core NPU utilization, 0-1.")
	fmt.Fprintln(w, "# TYPE mobilint_npu_core_utilization_ratio gauge")
	for _, x := range readings {
		if x.err != nil {
			continue
		}
		for _, c := range x.reading.Cores {
			if c.IntervalUs <= 0 {
				continue
			}
			fmt.Fprintf(w, "mobilint_npu_core_utilization_ratio{device=%q,cluster=%q,core=%q} %g\n",
				x.id, fmt.Sprintf("%d", c.Cluster), fmt.Sprintf("%d", c.Core),
				float64(c.NPUTimeUs)/float64(c.IntervalUs))
		}
	}
}

func emitInfoMetric(w io.Writer, readings []deviceReading) {
	fmt.Fprintln(w, "# HELP mobilint_npu_info Static ARIES device information.")
	fmt.Fprintln(w, "# TYPE mobilint_npu_info gauge")
	for _, x := range readings {
		if x.err != nil {
			continue
		}
		i := x.reading.Info
		fmt.Fprintf(
			w,
			"mobilint_npu_info{device=%q,model=%q,driver_version=%q,firmware_version=%q,firmware_crc=%q,vendor_id=%q,device_id=%q,sub_vendor_id=%q,sub_device_id=%q,pcie_gen=%q,pcie_lanes=%q,pcie_rev=%q,pcie_class_code=%q} 1\n",
			x.id,
			i.Model,
			i.DriverVersion,
			i.FirmwareVersion,
			fmt.Sprintf("%d", i.FirmwareCRC),
			fmt.Sprintf("0x%04x", i.VendorID),
			fmt.Sprintf("0x%04x", i.DeviceID),
			fmt.Sprintf("0x%04x", i.SubVendorID),
			fmt.Sprintf("0x%04x", i.SubDeviceID),
			fmt.Sprintf("%d", i.PCIeGen),
			fmt.Sprintf("%d", i.PCIeLanes),
			fmt.Sprintf("%d", i.PCIeRev),
			fmt.Sprintf("0x%08x", i.PCIeClassCode),
		)
	}
}

func emitMetric[T int32 | int64 | uint64 | float64](w io.Writer, readings []deviceReading, name, help string, get func(aries.Reading) T) {
	fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s gauge\n", name, help, name)
	for _, x := range readings {
		if x.err != nil {
			continue
		}
		fmt.Fprintf(w, "%s{device=%q} %v\n", name, x.id, get(x.reading))
	}
}

type processJSON struct {
	PID             int32   `json:"pid"`
	MemoryUsedBytes uint64  `json:"memory_used_bytes"`
	Utilization     float64 `json:"utilization"`
}

type deviceProcessesJSON struct {
	Device    string        `json:"device"`
	Error     string        `json:"error,omitempty"`
	Processes []processJSON `json:"processes"`
}

// handleProcesses serves current per-process detail as JSON, off the Prometheus
// path, so per-pid data is available on demand without the time-series
// cardinality that pid labels would add to /metrics.
func handleProcesses(w http.ResponseWriter, r *http.Request) {
	out := make([]deviceProcessesJSON, 0)
	for _, x := range collect() {
		d := deviceProcessesJSON{Device: x.id, Processes: []processJSON{}}
		if x.err != nil {
			d.Error = x.err.Error()
		}
		for _, p := range x.reading.Processes {
			util := 0.0
			if p.IntervalUs > 0 {
				util = float64(p.NPUTimeUs) / float64(p.IntervalUs)
			}
			d.Processes = append(d.Processes, processJSON{
				PID:             p.PID,
				MemoryUsedBytes: p.MemoryUsedBytes,
				Utilization:     util,
			})
		}
		out = append(out, d)
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(out); err != nil {
		klog.V(2).Infof("metrics: encode /process: %v", err)
	}
}
