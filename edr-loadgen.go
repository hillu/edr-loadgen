package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

type cpustat struct {
	utime float64
	stime float64
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

func main() {
	if e := os.Getenv("EDR_LOADGEN"); e != "" {
		os.Exit(0)
	}
	var (
		cmd, report     string
		delay, duration float64
	)
	var pids []uint64
	var rw *csv.Writer
	flag.StringVar(&cmd, "command", os.Args[0], "command to run")
	flag.Float64Var(&delay, "delay", .1, "delay between execs (in seconds)")
	flag.Float64Var(&duration, "duration", 60, "total duration (in seconds)")
	flag.StringVar(&report, "report", "", "report file (CSV)")
	flag.Parse()
	if len(flag.Args()) == 0 {
		fmt.Fprintln(os.Stderr, "No PIDs specified")
		os.Exit(1)
	}
	log.Printf("%s: exec '%s', every %.04f seconds, duration: %.04f seconds", os.Args[0], cmd, delay, duration)
	cmdlist := strings.Split(cmd, " ")
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
			log.Fatalf("parse: %s: %v", arg, err)
		}
		pids = append(pids, pid)
	}
	pnames := getNames(pids)

	var counter uint64
	t := time.NewTicker(time.Duration(delay * float64(time.Second)))
	procs := make(chan *os.Process, 100000)

	for i := 0; i < 32; i++ {
		go func(cmdlist []string, tick <-chan time.Time, procs chan *os.Process) {
			for range tick {
				p, err := os.StartProcess(cmdlist[0], cmdlist,
					&os.ProcAttr{
						Env:   []string{"EDR_LOADGEN=1"},
						Files: []*os.File{nil, nil, nil},
						Sys:   &syscall.SysProcAttr{},
					})
				if err != nil {
					log.Fatalf("exec: %v", err)
				}
				// p.Release()
				procs <- p
				atomic.AddUint64(&counter, 1)
			}
		}(cmdlist, t.C, procs)
		go func(procs chan *os.Process) {
			for p := range procs {
				p.Wait()
			}
		}(procs)
	}

	before, err := readStats(pids)
	if err != nil {
		log.Fatalf("read stats: %v", err)
	}
	time.Sleep(time.Duration(duration * float64(time.Second)))
	after, err := readStats(pids)
	if err != nil {
		log.Fatalf("read stats: %v", err)
	}

	t.Stop()
	if err != nil {
		log.Fatalf("read stats: %v", err)
	}
	log.Printf("%d events generated.", counter)
	var sec, perc float64
	now := time.Now().Unix()
	for _, pid := range pids {
		utime := after[pid].utime - before[pid].utime
		stime := after[pid].stime - before[pid].stime
		utimePerc := 100 * utime / duration
		stimePerc := 100 * stime / duration

		log.Printf("PID %d[%s]: %.02f+%.02f = %.02f seconds / %.3f+%.3f = %.03f percent",
			pid, pnames[pid], utime, stime, utime+stime,
			utimePerc, stimePerc, utimePerc+stimePerc,
		)
		if rw != nil {
			if err := rw.Write([]string{
				strconv.Itoa(int(now)),
				strconv.Itoa(int(duration / delay)),
				strconv.Itoa(int(counter)),
				strconv.Itoa(int(pid)),
				pnames[pid],
				strconv.FormatFloat(utime, 'f', 2, 64),
				strconv.FormatFloat(stime, 'f', 2, 64),
				strconv.FormatFloat(utime+stime, 'f', 2, 64),
				strconv.FormatFloat(utimePerc, 'f', 3, 64),
				strconv.FormatFloat(stimePerc, 'f', 3, 64),
				strconv.FormatFloat(utimePerc+stimePerc, 'f', 3, 64),
			}); err != nil {
				log.Printf("write report: %v", err)
			}
		}

		sec += utime + stime
		perc += utimePerc + stimePerc
	}
	if len(pids) > 1 {
		log.Printf("SUM: %.02f seconds / %.03f percent", sec, perc)
		if rw != nil {
			if err := rw.Write([]string{
				strconv.Itoa(int(now)),
				strconv.Itoa(int(duration / delay)),
				strconv.Itoa(int(counter)),
				"",
				"SUM",
				"", "",
				strconv.FormatFloat(sec, 'f', 2, 64),
				"", "",
				strconv.FormatFloat(perc, 'f', 3, 64),
			}); err != nil {
				log.Printf("write report: %v", err)
			}
		}
	}
}
