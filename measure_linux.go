package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"strconv"
	"strings"
)

// #include <unistd.h>
import "C"

var clk_tck = (uint64)(C.sysconf(C._SC_CLK_TCK))

func readStat(pid uint64) (*cpustat, error) {
	// see proc(5)
	buf, err := ioutil.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	parts := bytes.Split(buf, []byte(" "))
	if len(parts) < 15 {
		return nil, errors.New("wrong data in stat")
	}
	var utime, stime uint64
	if utime, err = strconv.ParseUint(string(parts[13]), 10, 64); err != nil {
		return nil, err
	}
	if stime, err = strconv.ParseUint(string(parts[14]), 10, 64); err != nil {
		return nil, err
	}
	return &cpustat{
		utime: float64(utime) / float64(clk_tck),
		stime: float64(stime) / float64(clk_tck),
	}, nil
}

func getNames(pids []uint64) map[uint64]string {
	pnames := make(map[uint64]string)
	for _, pid := range pids {
		if name, err := ioutil.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid)); err != nil {
			log.Printf("could not get cmdline for %d: %v", pid, err)
			pnames[pid] = "<unk>"
		} else {
			pnames[pid] = strings.TrimSpace(strings.Replace(string(name), "\x00", " ", -1))
		}
	}
	return pnames
}
