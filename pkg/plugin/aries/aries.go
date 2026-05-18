package aries

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/unix"
)

// ioctl ABI constants from aries_driver_ioctl.h.
const (
	ariesIOCMagic     = 'A'
	ariesNRDriverInfo = 1
	sizeofDriverInfo  = 128 // sizeof(struct aries_driver_info)
)

// Linux ioctl number encoding (asm-generic/ioctl.h).
const (
	iocNone      = 0
	iocRead      = 2
	iocDirShift  = 30
	iocSizeShift = 16
	iocTypeShift = 8
)

// ARIES_IOC_DRIVER_INFO = _IOR('A', 1, struct aries_driver_info)
const ariesIOCDriverInfo = (iocRead << iocDirShift) |
	(sizeofDriverInfo << iocSizeShift) |
	(ariesIOCMagic << iocTypeShift) |
	ariesNRDriverInfo

// _IO('A', N) — direction=none, size=0 → simplifies to (magic<<8) | N.
const (
	ariesIOCGetTemperature  = (ariesIOCMagic << iocTypeShift) | 7
	ariesIOCGetClockNPU     = (ariesIOCMagic << iocTypeShift) | 8
	ariesIOCGetClockNOC     = (ariesIOCMagic << iocTypeShift) | 9
	ariesIOCGetTotalPower   = (ariesIOCMagic << iocTypeShift) | 14
	ariesIOCGetTotalCurrent = (ariesIOCMagic << iocTypeShift) | 15
	ariesIOCGetFDCount      = (ariesIOCMagic << iocTypeShift) | 25
	ariesIOCGetTotalVoltage = (ariesIOCMagic << iocTypeShift) | 31
	ariesIOCGetFanDuty      = (ariesIOCMagic << iocTypeShift) | 32
)

// Sample is one snapshot of metric values for a single device.
// Values are raw integers returned by their respective ioctls; unit conversions
// (mW→W, Hz→MHz) are left to consumers / PromQL.
type Sample struct {
	Temperature  int32
	ClockNPUHz   int32
	ClockNOCHz   int32
	PowerTotal   int32
	CurrentTotal int32
	VoltageTotal int32
	FanDuty      int32
	FDCount      int32
}

func IsHealthy(path string) error {
	fd, err := unix.Open(path, unix.O_RDONLY, 0)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer unix.Close(fd)

	return driverInfoCheck(fd)
}

// ReadSample issues DRIVER_INFO (as health gate) then reads all metric ioctls.
func ReadSample(path string) (Sample, error) {
	fd, err := unix.Open(path, unix.O_RDONLY, 0)
	if err != nil {
		return Sample{}, fmt.Errorf("open %s: %w", path, err)
	}
	defer unix.Close(fd)

	if err := driverInfoCheck(fd); err != nil {
		return Sample{}, err
	}

	var s Sample
	reads := []struct {
		dst  *int32
		req  uintptr
		name string
	}{
		{&s.Temperature, ariesIOCGetTemperature, "GET_TEMPERATURE"},
		{&s.ClockNPUHz, ariesIOCGetClockNPU, "GET_CLOCK_NPU"},
		{&s.ClockNOCHz, ariesIOCGetClockNOC, "GET_CLOCK_NOC"},
		{&s.PowerTotal, ariesIOCGetTotalPower, "GET_TOTAL_POWER"},
		{&s.CurrentTotal, ariesIOCGetTotalCurrent, "GET_TOTAL_CURRENT"},
		{&s.VoltageTotal, ariesIOCGetTotalVoltage, "GET_TOTAL_VOLTAGE"},
		{&s.FanDuty, ariesIOCGetFanDuty, "GET_FAN_DUTY"},
		{&s.FDCount, ariesIOCGetFDCount, "GET_FD_COUNT"},
	}
	for _, r := range reads {
		v, err := ioctlReadInt(fd, r.req)
		if err != nil {
			return s, fmt.Errorf("%s: %w", r.name, err)
		}
		*r.dst = v
	}
	return s, nil
}

// TODO(further): change ioctl logic to mbltml, should be real health check
func driverInfoCheck(fd int) error {
	var buf [sizeofDriverInfo]byte
	_, _, errno := unix.Syscall(
		unix.SYS_IOCTL,
		uintptr(fd),
		uintptr(ariesIOCDriverInfo),
		uintptr(unsafe.Pointer(&buf[0])),
	)
	if errno != 0 {
		return fmt.Errorf("ARIES_IOC_DRIVER_INFO: %w", errno)
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
