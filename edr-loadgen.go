package main

import (
	"bytes"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

// #include <unistd.h>
import "C"

var clk_tck = (uint64)(C.sysconf(C._SC_CLK_TCK))

type cpustat struct {
	utime uint64
	stime uint64
}

func readStat(pid uint64) (*cpustat, error) {
	// see proc(5)
	buf, err := ioutil.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	parts := bytes.Split(buf, []byte(" "))
	if len(parts) < 15 {
		return nil, errors.New("wrong data in stat")
	}
	var cs cpustat
	if cs.utime, err = strconv.ParseUint(string(parts[13]), 10, 64); err != nil {
		return nil, err
	}
	if cs.stime, err = strconv.ParseUint(string(parts[14]), 10, 64); err != nil {
		return nil, err
	}
	return &cs, nil
}

func readStats(pids []uint64) (map[uint64]cpustat, error) {
	stats := make(map[uint64]cpustat)
	for _, pid := range pids {
		cs, err := readStat(pid)
		if err != nil {
			return nil, err
		}
		stats[pid] = *cs
	}
	return stats, nil
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

func main() {
	var (
		cmd, report     string
		delay, duration float64
	)
	var pids []uint64
	var rw *csv.Writer
	flag.StringVar(&cmd, "command", "/bin/true", "command to run")
	flag.Float64Var(&delay, "delay", .1, "delay between execs (in seconds)")
	flag.Float64Var(&duration, "duration", 60, "total duration (in seconds)")
	flag.StringVar(&report, "report", "", "report file (CSV)")
	flag.Parse()
	if len(flag.Args()) == 0 {
		fmt.Fprintln(os.Stderr, "No PIDs specified")
		os.Exit(1)
	}
	log.Printf("%s: exec '%s', every %.04f seconds, duration: %.04f seconds", os.Args[0], cmd, delay, duration)
	log.Printf("CLK_TCK = %d", clk_tck)
	if delay == .0 {
		log.Fatal("delay cannot be 0")
	}
	if delay > duration/10 {
		log.Fatal("delay must be much smaller than duration")
	}
	if report != "" {
		f, err := os.OpenFile(report, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			log.Fatalf("could not open report file: %v", err)
		}
		rw = csv.NewWriter(f)
		defer func(f *os.File, rw *csv.Writer) {
			rw.Flush()
			f.Close()
		}(f, rw)
	}
	for _, arg := range flag.Args() {
		pid, err := strconv.ParseUint(arg, 10, 64)
		if err != nil {
			log.Fatal("parse: %s: %v", arg, err)
		}
		pids = append(pids, pid)
	}
	pnames := getNames(pids)

	var counter uint64
	t := time.NewTicker(time.Duration(delay * float64(time.Second)))
	procs := make(chan *os.Process, 100000)

	for i := 0; i < 32; i++ {
		go func(cmd string, tick <-chan time.Time, procs chan *os.Process) {
			for range tick {
				p, err := os.StartProcess(cmd, []string{cmd}, &os.ProcAttr{})
				if err != nil {
					log.Fatalf("exec: %v", err)
				}
				// p.Release()
				procs <- p
				atomic.AddUint64(&counter, 1)
			}
		}(cmd, t.C, procs)
		go func(procs chan *os.Process) {
			for p := range procs {
				p.Wait()
			}
		}(procs)
	}

	before, err := readStats(pids)
	if err != nil {
		log.Fatal("read stats: %v", err)
	}
	time.Sleep(time.Duration(duration * float64(time.Second)))
	after, err := readStats(pids)
	if err != nil {
		log.Fatal("read stats: %v", err)
	}

	t.Stop()
	if err != nil {
		log.Fatal("read stats: %v", err)
	}
	log.Printf("%d events generated.", counter)
	var sec, perc float64
	now := time.Now().Unix()
	for _, pid := range pids {
		utime := after[pid].utime - before[pid].utime
		stime := after[pid].stime - before[pid].stime
		utimeSec := float64(utime) / float64(clk_tck)
		stimeSec := float64(stime) / float64(clk_tck)
		utimePerc := 100 * utimeSec / duration
		stimePerc := 100 * stimeSec / duration

		log.Printf("PID %d[%s]: user+sys: %d+%d = %d ticks / %.02f+%.02f = %.02f seconds / %.3f+%.3f = %.03f percent",
			pid, pnames[pid], utime, stime, utime+stime,
			utimeSec, stimeSec, utimeSec+stimeSec,
			utimePerc, stimePerc, utimePerc+stimePerc,
		)
		if rw != nil {
			if err := rw.Write([]string{
				strconv.Itoa(int(now)),
				strconv.FormatFloat(duration/delay, 'f', 2, 64),
				strconv.Itoa(int(counter)),
				strconv.Itoa(int(pid)), pnames[pid],
				strconv.Itoa(int(utime)),
				strconv.Itoa(int(stime)),
				strconv.Itoa(int(utime + stime)),
				strconv.FormatFloat(utimeSec, 'f', 2, 64),
				strconv.FormatFloat(stimeSec, 'f', 2, 64),
				strconv.FormatFloat(utimeSec+stimeSec, 'f', 2, 64),
				strconv.FormatFloat(utimePerc, 'f', 3, 64),
				strconv.FormatFloat(stimePerc, 'f', 3, 64),
				strconv.FormatFloat(utimePerc+stimePerc, 'f', 3, 64),
			}); err != nil {
				log.Printf("write report: %v", err)
			}
		}

		sec += utimeSec + stimeSec
		perc += utimePerc + stimePerc
	}
	if len(pids) > 1 {
		log.Printf("SUM: %.02f seconds / %.03f percent", sec, perc)
	}
}
