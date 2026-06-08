package aries

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

type Reading struct {
	Temperature      int32
	ClockNPUHz       int32
	ClockNOCHz       int32
	PowerTotal       float64
	CurrentTotal     float64
	VoltageTotal     float64
	FanDuty          int32
	FDCount          int32
	UtilizationRatio float64 // total NPU utilization 0-1
	Info             DeviceInfo
	Processes        []ProcessInfo
	Cores            []CoreInfo
}

type DeviceInfo struct {
	Model            string
	DriverVersion    string
	FirmwareVersion  string
	FirmwareCRC      int32
	VendorID         uint16
	DeviceID         uint16
	SubVendorID      uint16
	SubDeviceID      uint16
	PCIeGen          uint32
	PCIeLanes        uint32
	PCIeRev          uint32
	PCIeClassCode    uint32
	MemoryTotalBytes int64
}

type ProcessInfo struct {
	PID             int32
	MemoryUsedBytes uint64
	NPUTimeUs       int64
	IntervalUs      int64
}

type CoreInfo struct {
	Cluster    int
	Core       int
	NPUTimeUs  int64
	IntervalUs int64
}

func (s Reading) MemoryUsedBytes() uint64 {
	var total uint64
	for _, p := range s.Processes {
		total += p.MemoryUsedBytes
	}
	return total
}

func deviceNumber(path string) (int, error) {
	id := filepath.Base(path)
	raw, ok := strings.CutPrefix(id, "aries")
	if !ok || raw == "" {
		return 0, fmt.Errorf("unexpected aries device path %s", path)
	}

	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("unexpected aries device path %s: %w", path, err)
	}
	return n, nil
}
