//go:build !linux && !windows

package server

type syscallStat struct {
	total uint64
	used  uint64
	free  uint64
}

func (s *syscallStat) get(path string) error {
	return nil
}

func getTotalMemory() uint64 {
	return 0
}

func getFreeMemory() uint64 {
	return 0
}

func getUsedMemory() uint64 {
	return 0
}
