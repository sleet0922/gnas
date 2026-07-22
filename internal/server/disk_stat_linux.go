package server

import (
	"syscall"
)

type syscallStat struct {
	total uint64
	used  uint64
	free  uint64
}

func (s *syscallStat) get(path string) error {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return err
	}
	s.total = stat.Blocks * uint64(stat.Bsize)
	s.free = stat.Bavail * uint64(stat.Bsize)
	s.used = s.total - stat.Bfree*uint64(stat.Bsize)
	return nil
}
