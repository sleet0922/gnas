//go:build windows

package server

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

type syscallStat struct {
	total uint64
	used  uint64
	free  uint64
}

func (s *syscallStat) get(path string) error {
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	var freeBytes, totalBytes, totalFreeBytes uint64
	err = windows.GetDiskFreeSpaceEx(pathPtr, &freeBytes, &totalBytes, &totalFreeBytes)
	if err != nil {
		return err
	}
	s.total = totalBytes
	s.free = freeBytes
	s.used = totalBytes - freeBytes
	return nil
}

// MEMORYSTATUSEX is the Windows structure for memory statistics
type MEMORYSTATUSEX struct {
	Length     uint32
	MemoryLoad uint32
	TotalPhys  uint64
	AvailPhys  uint64
	TotalPage  uint64
	AvailPage  uint64
	TotalVirt  uint64
	AvailVirt  uint64
	AvailExt   uint64
}

func getWindowsMemory() (total uint64, free uint64, err error) {
	kernel32 := windows.NewLazySystemDLL("kernel32.dll")
	procGlobalMemoryStatusEx := kernel32.NewProc("GlobalMemoryStatusEx")

	var ms MEMORYSTATUSEX
	ms.Length = uint32(unsafe.Sizeof(ms))

	r1, _, err := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&ms)))
	if r1 == 0 {
		return 0, 0, err
	}
	return ms.TotalPhys, ms.AvailPhys, nil
}

func getTotalMemory() uint64 {
	total, _, _ := getWindowsMemory()
	return total
}

func getFreeMemory() uint64 {
	_, free, _ := getWindowsMemory()
	return free
}

func getUsedMemory() uint64 {
	total, free, _ := getWindowsMemory()
	return total - free
}
