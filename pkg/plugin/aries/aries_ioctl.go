package aries

import (
	"encoding/binary"
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	ariesIOCMagic     = 'A'
	ariesNRDriverInfo = 1
	sizeofDriverInfo  = 128 // sizeof(struct aries_driver_info)
)

// Linux ioctl number encoding (asm-generic/ioctl.h).
const (
	iocWrite     = 1
	iocRead      = 2
	iocDirShift  = 30
	iocSizeShift = 16
	iocTypeShift = 8
)

const (
	ariesNumCores         = 5
	ariesNumAllCores      = 2 * ariesNumCores // 2 clusters x 5 cores
	ariesTimeQueueEntries = 128
	oneSecUs              = 1_000_000
)

// compute cores, excluding the per-cluster global slot (index 0 and 5).
var ariesLocalCoreIndices = []int{1, 2, 3, 4, 6, 7, 8, 9}

// ARIES_IOC_DRIVER_INFO = _IOR('A', 1, struct aries_driver_info)
const ariesIOCDriverInfo = (iocRead << iocDirShift) |
	(sizeofDriverInfo << iocSizeShift) |
	(ariesIOCMagic << iocTypeShift) |
	ariesNRDriverInfo

const (
	ariesIOCGetCRC          = (ariesIOCMagic << iocTypeShift) | 11
	ariesIOCGetTemperature  = (ariesIOCMagic << iocTypeShift) | 7
	ariesIOCGetClockNPU     = (ariesIOCMagic << iocTypeShift) | 8
	ariesIOCGetClockNOC     = (ariesIOCMagic << iocTypeShift) | 9
	ariesIOCGetTotalPower   = (ariesIOCMagic << iocTypeShift) | 14
	ariesIOCGetTotalCurrent = (ariesIOCMagic << iocTypeShift) | 15
	ariesIOCGetFDCount      = (ariesIOCMagic << iocTypeShift) | 25
	ariesIOCGetTotalVoltage = (ariesIOCMagic << iocTypeShift) | 31
	ariesIOCGetFanDuty      = (ariesIOCMagic << iocTypeShift) | 32
	ariesIOCCaptureSnapshot = (ariesIOCMagic << iocTypeShift) | 37
	ariesIOCReleaseSnapshot = (ariesIOCMagic << iocTypeShift) | 38
)

const (
	ariesSnapshotResultSize = 24
	ariesIOCReadSnapshot    = (iocRead << iocDirShift) |
		(ariesSnapshotResultSize << iocSizeShift) |
		(ariesIOCMagic << iocTypeShift) |
		39
)

type ariesSnapshotResult struct {
	EOS  bool
	_    [7]byte
	Info ariesProcessInfo
}

type ariesProcessInfo struct {
	PID     int32
	Count   int32
	MemUsed uint64
}

// ARIES_IOC_GET_MONITOR_INFO = _IOWR('A', 23, struct aries_core_monitor_req)
const (
	sizeofCoreMonitorReq   = 48 // 8-byte pointer + int[10]
	ariesIOCGetMonitorInfo = ((iocRead | iocWrite) << iocDirShift) |
		(sizeofCoreMonitorReq << iocSizeShift) |
		(ariesIOCMagic << iocTypeShift) |
		23
)

type ariesTimeEst struct {
	BeginTimeUs int64
	EndTimeUs   int64
	PID         int32
	_           int32 // pad to 24-byte stride matching the C struct
}

type ariesCoreMonitorReq struct {
	NpuTsUs *ariesTimeEst
	IdxHead [ariesNumAllCores]int32
}

func IsHealthy(path string) error {
	fd, err := unix.Open(path, unix.O_RDONLY, 0)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer unix.Close(fd)

	return driverInfoCheck(fd)
}

// Read issues DRIVER_INFO as a health gate, then reads all metrics with
// the same ARIES ioctl ABI mbltml uses internally.
func Read(path string) (Reading, error) {
	fd, err := unix.Open(path, unix.O_RDONLY, 0)
	if err != nil {
		return Reading{}, fmt.Errorf("open %s: %w", path, err)
	}
	defer unix.Close(fd)

	info, err := readDeviceInfo(fd)
	if err != nil {
		return Reading{}, err
	}

	s := Reading{Info: info}
	reads := []struct {
		set  func(int32)
		req  uintptr
		name string
	}{
		{func(v int32) { s.Temperature = v }, ariesIOCGetTemperature, "GET_TEMPERATURE"},
		{func(v int32) { s.ClockNPUHz = v }, ariesIOCGetClockNPU, "GET_CLOCK_NPU"},
		{func(v int32) { s.ClockNOCHz = v }, ariesIOCGetClockNOC, "GET_CLOCK_NOC"},
		{func(v int32) { s.PowerTotal = float64(v) }, ariesIOCGetTotalPower, "GET_TOTAL_POWER"},
		{func(v int32) { s.CurrentTotal = float64(v) }, ariesIOCGetTotalCurrent, "GET_TOTAL_CURRENT"},
		{func(v int32) { s.VoltageTotal = float64(v) }, ariesIOCGetTotalVoltage, "GET_TOTAL_VOLTAGE"},
		{func(v int32) { s.FanDuty = v }, ariesIOCGetFanDuty, "GET_FAN_DUTY"},
		{func(v int32) { s.FDCount = v }, ariesIOCGetFDCount, "GET_FD_COUNT"},
	}
	for _, r := range reads {
		v, err := ioctlReadInt(fd, r.req)
		if err != nil {
			return s, fmt.Errorf("%s: %w", r.name, err)
		}
		r.set(v)
	}
	// Firmware CRC is best-effort: the rev1 header marks GET_CRC as deprecated,
	// so a failure must not drop the whole sample.
	s.Info.FirmwareCRC = -1
	if v, err := ioctlReadInt(fd, ariesIOCGetCRC); err == nil {
		s.Info.FirmwareCRC = v
	}

	npuTimes, intervals := readMonitorInfo(fd, monotonicUs())
	s.Processes = readProcesses(fd, npuTimes, intervals)
	s.Cores = buildCoreInfos(npuTimes, intervals)
	s.UtilizationRatio = totalUtilization(s.Processes)
	return s, nil
}

func monotonicUs() int64 {
	var ts unix.Timespec
	if err := unix.ClockGettime(unix.CLOCK_MONOTONIC, &ts); err != nil {
		return 0
	}
	return int64(ts.Sec)*oneSecUs + int64(ts.Nsec)/1000
}

// readMonitorInfo runs ARIES_IOC_GET_MONITOR_INFO and reduces the per-core
// timestamp ring buffers into per-pid NPU time and per-core sampling intervals,
// mirroring mbltml's calculateNPUTimeAndInterval. Returns (nil, nil) if the
// ioctl is unavailable.
func readMonitorInfo(fd int, curr int64) (map[int32][]int64, []int64) {
	buf := make([]ariesTimeEst, ariesTimeQueueEntries*ariesNumAllCores)
	req := ariesCoreMonitorReq{NpuTsUs: &buf[0]}
	if _, _, errno := unix.Syscall(
		unix.SYS_IOCTL,
		uintptr(fd),
		uintptr(ariesIOCGetMonitorInfo),
		uintptr(unsafe.Pointer(&req)),
	); errno != 0 {
		return nil, nil
	}

	npuTimes := make(map[int32][]int64)
	intervals := make([]int64, ariesNumAllCores)
	tsIdx := func(head, order int) int {
		v := head - order
		if v < 0 {
			v += ariesTimeQueueEntries
		}
		return v
	}
	for i := 0; i < ariesNumAllCores; i++ {
		head := int(req.IdxHead[i])
		first := buf[i*ariesTimeQueueEntries+tsIdx(head, 0)]
		second := buf[i*ariesTimeQueueEntries+tsIdx(head, 1)]
		if first.BeginTimeUs < second.BeginTimeUs {
			head = tsIdx(head, 1)
		}
		oldest := curr
		j := 0
		for j < ariesTimeQueueEntries {
			ts := buf[i*ariesTimeQueueEntries+tsIdx(head, j)]
			if _, ok := npuTimes[ts.PID]; !ok {
				npuTimes[ts.PID] = make([]int64, ariesNumAllCores)
			}
			if curr-ts.BeginTimeUs > oneSecUs {
				if ts.BeginTimeUs != 0 {
					end := curr
					if ts.BeginTimeUs <= ts.EndTimeUs {
						end = ts.EndTimeUs
					}
					if elapsed := end - (curr - oneSecUs); elapsed > 0 {
						npuTimes[ts.PID][i] += elapsed
					}
				}
				break
			}
			oldest = ts.BeginTimeUs
			end := curr
			if ts.BeginTimeUs < ts.EndTimeUs {
				end = ts.EndTimeUs
			}
			npuTimes[ts.PID][i] += end - ts.BeginTimeUs
			j++
		}
		if j < ariesTimeQueueEntries {
			intervals[i] = oneSecUs
		} else {
			intervals[i] = curr - oldest
		}
	}
	return npuTimes, intervals
}

func buildCoreInfos(npuTimes map[int32][]int64, intervals []int64) []CoreInfo {
	if intervals == nil {
		return nil
	}
	cores := make([]CoreInfo, 0, len(ariesLocalCoreIndices))
	for _, ci := range ariesLocalCoreIndices {
		var t int64
		for _, times := range npuTimes {
			t += times[ci]
		}
		cores = append(cores, CoreInfo{
			Cluster:    ci / ariesNumCores,
			Core:       ci % ariesNumCores,
			NPUTimeUs:  t,
			IntervalUs: intervals[ci],
		})
	}
	return cores
}

// totalUtilization sums per-process utilization, clamped to 1.0. Returns -1 if
// any process lacks interval data (monitor info unavailable), matching mbltml.
func totalUtilization(procs []ProcessInfo) float64 {
	util := 0.0
	for _, p := range procs {
		if p.IntervalUs == 0 {
			return -1
		}
		util += float64(p.NPUTimeUs) / float64(p.IntervalUs)
	}
	if util > 1 {
		util = 1
	}
	return util
}

func driverInfoCheck(fd int) error {
	_, err := readDriverInfo(fd)
	return err
}

func readDriverInfo(fd int) ([sizeofDriverInfo]byte, error) {
	var buf [sizeofDriverInfo]byte
	_, _, errno := unix.Syscall(
		unix.SYS_IOCTL,
		uintptr(fd),
		uintptr(ariesIOCDriverInfo),
		uintptr(unsafe.Pointer(&buf[0])),
	)
	if errno != 0 {
		return buf, fmt.Errorf("ARIES_IOC_DRIVER_INFO: %w", errno)
	}
	return buf, nil
}

func readDeviceInfo(fd int) (DeviceInfo, error) {
	buf, err := readDriverInfo(fd)
	if err != nil {
		return DeviceInfo{}, err
	}

	card0 := binary.LittleEndian.Uint32(buf[0:4])
	card1 := binary.LittleEndian.Uint32(buf[4:8])
	card2 := binary.LittleEndian.Uint32(buf[8:12])
	pcie0 := binary.LittleEndian.Uint32(buf[16:20])
	pcie1 := binary.LittleEndian.Uint32(buf[20:24])
	driver0 := binary.LittleEndian.Uint32(buf[32:36])
	driver1 := binary.LittleEndian.Uint32(buf[36:40])

	// In aries_driver_info.card the leading reserved:32 bitfield consumes m[0],
	// so firmware version / dram_type / device_type / revision live in m[1]
	// (card1, aliased onto the PCIe subsystem ID) and version_patch in m[2] (card2).
	fwMajor := (card1 >> 10) & 0x3f
	fwMinor := card1 & 0x3ff
	fwPatch := uint32(0)
	if (card2>>31)&0x1 == 1 {
		fwPatch = (card2 >> 16) & 0x7fff
	}
	revision := (card1 >> 24) & 0xf
	dramType := (card1 >> 16) & 0x7
	memoryTotalBytes := int64((uint64(1) << (dramType + 1)) * 1024 * 1024 * 1024)

	return DeviceInfo{
		Model:            "ARIES",
		DriverVersion:    fmt.Sprintf("%d.%d.%d+rev%d", driver0&0xffff, driver0>>16, driver1&0xffff, driver1>>16),
		FirmwareVersion:  fmt.Sprintf("%d.%d.%d+rev%d", fwMajor, fwMinor, fwPatch, revision),
		VendorID:         uint16(card0),
		DeviceID:         uint16(card0 >> 16),
		SubVendorID:      uint16(card1),
		SubDeviceID:      uint16(card1 >> 16),
		PCIeGen:          pcie0 & 0xff,
		PCIeLanes:        (pcie0 >> 8) & 0xff,
		PCIeRev:          (pcie0 >> 16) & 0xff,
		PCIeClassCode:    pcie1,
		MemoryTotalBytes: memoryTotalBytes,
	}, nil
}

func readProcesses(fd int, npuTimes map[int32][]int64, intervals []int64) []ProcessInfo {
	if err := ioctlNoArg(fd, ariesIOCCaptureSnapshot); err != nil {
		return nil
	}
	defer ioctlNoArg(fd, ariesIOCReleaseSnapshot)

	self := int32(os.Getpid()) // Skip our own fd
	var out []ProcessInfo
	for {
		var r ariesSnapshotResult
		_, _, errno := unix.Syscall(
			unix.SYS_IOCTL,
			uintptr(fd),
			uintptr(ariesIOCReadSnapshot),
			uintptr(unsafe.Pointer(&r)),
		)
		if errno != 0 || r.EOS {
			break
		}
		if r.Info.PID <= 0 || r.Info.PID == self {
			continue
		}
		p := ProcessInfo{
			PID:             r.Info.PID,
			MemoryUsedBytes: r.Info.MemUsed,
		}

		for _, ci := range ariesLocalCoreIndices {
			if intervals != nil {
				p.IntervalUs += intervals[ci]
			}
			if t, ok := npuTimes[p.PID]; ok {
				p.NPUTimeUs += t[ci]
			}
		}
		out = append(out, p)
	}
	return out
}

func ioctlNoArg(fd int, req uintptr) error {
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), req, 0)
	if errno != 0 {
		return errno
	}
	return nil
}

// ioctlReadInt issues an _IO ioctl whose return value carries the result.
func ioctlReadInt(fd int, req uintptr) (int32, error) {
	r1, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), req, 0)
	if errno != 0 {
		return 0, errno
	}
	return int32(r1), nil
}
