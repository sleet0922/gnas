package server

import "syscall"

func getTotalMemory() uint64 {
	in := new(syscall.Sysinfo_t)
	err := syscall.Sysinfo(in)
	if err != nil {
		return 0
	}
	return in.Totalram * uint64(in.Unit)
}

func getFreeMemory() uint64 {
	in := new(syscall.Sysinfo_t)
	err := syscall.Sysinfo(in)
	if err != nil {
		return 0
	}
	return in.Freeram * uint64(in.Unit)
}

func getUsedMemory() uint64 {
	in := new(syscall.Sysinfo_t)
	err := syscall.Sysinfo(in)
	if err != nil {
		return 0
	}
	return (in.Totalram - in.Freeram) * uint64(in.Unit)
}
