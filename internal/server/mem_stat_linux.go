package server

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"syscall"
)

func readLinuxMemory() (total, available uint64, ok bool) {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0, false
	}
	defer file.Close()

	values := make(map[string]uint64, 2)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		if fields[0] != "MemTotal:" && fields[0] != "MemAvailable:" {
			continue
		}
		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			continue
		}
		// /proc/meminfo reports these values in kB.
		if len(fields) >= 3 && strings.EqualFold(fields[2], "kb") {
			value *= 1024
		}
		values[fields[0]] = value
	}
	if scanner.Err() != nil {
		return 0, 0, false
	}
	total, totalOK := values["MemTotal:"]
	available, availableOK := values["MemAvailable:"]
	return total, available, totalOK && availableOK && available <= total
}

func getTotalMemory() uint64 {
	if total, _, ok := readLinuxMemory(); ok {
		return total
	}
	in := new(syscall.Sysinfo_t)
	err := syscall.Sysinfo(in)
	if err != nil {
		return 0
	}
	return in.Totalram * uint64(in.Unit)
}

func getFreeMemory() uint64 {
	if _, available, ok := readLinuxMemory(); ok {
		return available
	}
	in := new(syscall.Sysinfo_t)
	err := syscall.Sysinfo(in)
	if err != nil {
		return 0
	}
	return in.Freeram * uint64(in.Unit)
}

func getUsedMemory() uint64 {
	if total, available, ok := readLinuxMemory(); ok {
		return total - available
	}
	in := new(syscall.Sysinfo_t)
	err := syscall.Sysinfo(in)
	if err != nil {
		return 0
	}
	return (in.Totalram - in.Freeram) * uint64(in.Unit)
}
