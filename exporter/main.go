package main

import (
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/procfs"
)

func matchProcess(p *procfs.Proc, pst *procfs.ProcStat, name string) (bool, error) {
	var (
		tmp     string
		err     error
		cmdline []string
	)
	if pst.Comm == name || strings.HasSuffix(pst.Comm, "/"+name) {
		return true, nil
	}
	if tmp, err = p.Executable(); err != nil {
		return false, err
	}
	if tmp == name || strings.HasSuffix(tmp, "/"+name) {
		return true, nil
	}
	if cmdline, err = p.CmdLine(); err != nil {
		return false, err
	}
	for i, tmp := range cmdline {
		if i > 2 {
			break
		}
		if tmp == name {
			return true, nil
		}
	}
	return false, nil
}

func recordMetrics() {
	var (
		err         error
		truePath    string
		lastNodePid int64
	)
	truePath, err = exec.LookPath("true")
	if err != nil {
		log.Printf("Failed to look up the true binary: %v\n", err)
		os.Exit(1)
	}
	go func() {
		var (
			cmd *exec.Cmd
		)
		for {
			cmd = exec.Command(truePath)
			if err := cmd.Start(); err != nil {
				log.Printf("Failed to execute true: %v\n", err)
				time.Sleep(2 * time.Second)
				continue
			}
			opsHighestPid.Set(float64(cmd.Process.Pid))
			if lastNodePid > int64(cmd.Process.Pid) {
				log.Printf("PID overflow occured: prevPID=%d, newPID=%d\n", lastNodePid, cmd.Process.Pid)
				opsPidOverflow.Inc()
			}
			opsProcessed.Inc()
			if err := cmd.Wait(); err != nil {
				log.Printf("Failed to wait on pid %d\n: %v", cmd.Process.Pid, err)
			}
			time.Sleep(2 * time.Second)
		}
	}()

	go func() {
		var pidMax int64
		for {
			b, err := ioutil.ReadFile("/proc/sys/kernel/pid_max")
			if err != nil {
				log.Printf("Failed to read /proc/sys/kernel/pid_max: %v\n", err)
				time.Sleep(10 * time.Second)
				continue
			}
			pidMax, err = strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64)
			if err != nil {
				log.Printf("Failed to parse pid_max \"%s\" as a number: %v\n", string(b), err)
				time.Sleep(10 * time.Second)
				continue
			}
			opsMaxPid.Set(float64(pidMax))
			time.Sleep(10 * time.Second)
		}
	}()

	go func() {
		for {
			procs, err := procfs.AllProcs()
			matchCmds := []string{"conmom", "curl"}
			var counters = map[string]uint{}
			if err != nil {
				log.Printf("Failed to get processes: %v\n", err)
				continue
			}
			evalFailed := 0
		procsLoop:
			for _, p := range procs {
				pst, err := p.Stat()
				if err != nil {
					//log.Printf("Failed to get proc stat for PID=%d: %v\n", p.PID, err)
					evalFailed++
					continue
				}
				if pst.State != "Z" {
					continue
				}
				other := true
				for _, cmd := range matchCmds {
					if b, err := matchProcess(&p, &pst, cmd); err != nil {
						log.Printf("Failed to get cmdline info for a zombie PID=%d: %v\n", p.PID, err)
						evalFailed++
						continue procsLoop
					} else if b {
						counters[cmd]++
						other = false
						break
					}
				}
				if other {
					counters["other"]++
				}
			}

			for _, cmd := range matchCmds {
				opsZombies.With(prometheus.Labels{"cmd": cmd}).Set(float64(counters[cmd]))
			}
			opsZombies.With(prometheus.Labels{"cmd": "other"}).Set(float64(counters["other"]))
			if evalFailed > 0 {
				//log.Printf("Failed to evaluate %d processes!", evalFailed)
				opsProcEvalFailed.Add(float64(evalFailed))
			}
			time.Sleep(5 * time.Second)
		}
	}()
}

var (
	// PID metrics
	opsProcessed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "node_latest_pid_count",
		Help: "The total number of pid scrapes performed. Each scrape itself increments the latest pid by one.",
	})
	opsHighestPid = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "node_latest_pid",
		Help: "The latest process PID generated. Kernel's PID counter increments by one each time a new" + " process is created.",
	})
	opsMaxPid = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "node_max_pids",
		Help: "Maximum PIDs on the node. Number around which the linux PIDs wrap.",
	})
	opsPidOverflow = promauto.NewCounter(prometheus.CounterOpts{
		Name: "node_pid_overflow_count",
		Help: "Number of times PID counter overflow has been detected.",
	})

	// Process metrics
	opsZombies = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "node_zombies",
		Help: "Number of zombie processes on the node.",
	}, []string{"cmd"})
	opsProcEvalFailed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "node_zombies_eval_failed",
		Help: "Number of processes that failed to be evaluated.",
	})
)

func main() {
	recordMetrics()

	http.Handle("/metrics", promhttp.Handler())
	http.ListenAndServe(":2112", nil)
}
