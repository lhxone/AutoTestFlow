//go:build windows

package service

import (
	"path/filepath"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

type memoryStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

type filetime struct {
	LowDateTime  uint32
	HighDateTime uint32
}

func getDiskInfo() (DiskInfo, error) {
	workspaceRoot := getWorkspaceRoot()

	volumeName := filepath.VolumeName(workspaceRoot)
	if volumeName == "" {
		volumeName = "C:"
	}
	rootPath := volumeName + `\\`
	rootPathPtr, err := windows.UTF16PtrFromString(rootPath)
	if err != nil {
		return DiskInfo{}, err
	}

	var freeBytes, totalBytes, totalFree uint64
	err = windows.GetDiskFreeSpaceEx(rootPathPtr, &freeBytes, &totalBytes, &totalFree)
	if err != nil {
		return DiskInfo{}, err
	}

	usedPct := 0.0
	if totalBytes > 0 {
		usedPct = float64(totalBytes-freeBytes) / float64(totalBytes) * 100
	}

	return DiskInfo{
		TotalBytes:     totalBytes,
		FreeBytes:      freeBytes,
		UsedPercentage: usedPct,
	}, nil
}

func getMemoryInfo() (*memoryInfo, error) {
	mem := &memoryStatusEx{Length: uint32(unsafe.Sizeof(memoryStatusEx{}))}
	r1, _, err := windows.NewLazyDLL("kernel32.dll").
		NewProc("GlobalMemoryStatusEx").
		Call(uintptr(unsafe.Pointer(mem)))
	if r1 == 0 {
		return nil, err
	}
	return &memoryInfo{
		TotalBytes: mem.TotalPhys,
		UsedBytes:  mem.TotalPhys - mem.AvailPhys,
		Percentage: float64(mem.TotalPhys-mem.AvailPhys) / float64(mem.TotalPhys) * 100,
	}, nil
}

func getCPUPercentage() float64 {
	kernel32 := windows.NewLazyDLL("kernel32.dll")
	getSystemTimes := kernel32.NewProc("GetSystemTimes")

	var idle1, kernel1, user1 filetime
	getSystemTimes.Call(
		uintptr(unsafe.Pointer(&idle1)),
		uintptr(unsafe.Pointer(&kernel1)),
		uintptr(unsafe.Pointer(&user1)),
	)

	time.Sleep(500 * time.Millisecond)

	var idle2, kernel2, user2 filetime
	getSystemTimes.Call(
		uintptr(unsafe.Pointer(&idle2)),
		uintptr(unsafe.Pointer(&kernel2)),
		uintptr(unsafe.Pointer(&user2)),
	)

	idleDelta := ftToUint64(idle2) - ftToUint64(idle1)
	kernelDelta := ftToUint64(kernel2) - ftToUint64(kernel1)
	userDelta := ftToUint64(user2) - ftToUint64(user1)

	totalDelta := kernelDelta + userDelta
	busyDelta := (kernelDelta - idleDelta) + userDelta
	if totalDelta == 0 {
		return 0
	}
	return float64(busyDelta) / float64(totalDelta) * 100
}

func ftToUint64(ft filetime) uint64 {
	return uint64(ft.HighDateTime)<<32 | uint64(ft.LowDateTime)
}
