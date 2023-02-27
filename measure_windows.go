package main

import (
	"syscall"

	"github.com/elastic/go-windows"
)

func readStat(pid uint64) (*cpustat, error) {
	h, err := syscall.OpenProcess(syscall.PROCESS_QUERY_INFORMATION, false, uint32(pid))
	if err != nil {
		return nil, err
	}
	defer syscall.CloseHandle(h)
	var userTime, kernelTime syscall.Filetime
	if err := syscall.GetProcessTimes(h, nil, nil, &kernelTime, &userTime); err != nil {
		return nil, err
	}
	return &cpustat{
		utime: float64(userTime.Nanoseconds()) / 1000000000,
		stime: float64(kernelTime.Nanoseconds()) / 1000000000,
	}, nil
}

func getNames(pids []uint64) map[uint64]string {
	pnames := make(map[uint64]string)
	for _, pid := range pids {
		h, err := syscall.OpenProcess(syscall.PROCESS_QUERY_INFORMATION, false, uint32(pid))
		if err != nil {
			pnames[pid] = "<unknown>"
		} else if s, err := windows.GetProcessImageFileName(h); err != nil {
			pnames[pid] = "<unknown>"
		} else {
			pnames[pid] = s
		}
		syscall.CloseHandle(h)
	}
	return pnames
}
