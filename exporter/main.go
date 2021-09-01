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

var (
	// const
	matchCmds = []string{"conmom", "curl"}
)

type zombieInfo struct {
	cmd  string
	pcmd string
}
type pidT uint64
type pid2cmdT map[pidT]string

func getZombieInfo(p *procfs.Proc, pst *procfs.ProcStat, pidCache *pid2cmdT) (zombieInfo, error) {
	var err error
	zi := zombieInfo{
		cmd: "other",
	}
	for _, cmd := range matchCmds {
		if b, _err := matchProcess(p, pst, cmd); _err != nil {
			log.Printf("Failed to get cmdline info for a zombie PID=%d: %v\n", p.PID, err)
			err = _err
			break
		} else if b {
			zi.cmd = cmd
			break
		}
	}
	if cmd, ok := (*pidCache)[pidT(pst.PPID)]; ok {
		zi.pcmd = cmd
		return zi, err
	}

	var (
		pp   procfs.Proc
		ppst procfs.ProcStat
	)
	pp, err = procfs.NewProc(pst.PPID)
	if err != nil {
		log.Printf("Failed to get parent process info for pid=%d (ppid=%d): %v\n", p.PID, pst.PPID, err)
		return zi, err
	}
	ppst, err = pp.Stat()
	if err != nil {
		log.Printf("Failed to get parent process stats for pid=%d (ppid=%d): %v\n", p.PID, pst.PPID, err)
		return zi, err
	}

	if tmp, _err := pp.CmdLine(); _err == nil && len(tmp) > 0 && tmp[0] != "" {
		zi.pcmd = tmp[0]
		if i := strings.LastIndex(zi.pcmd, "/"); i >= 0 {
			zi.pcmd = zi.pcmd[i+1:]
		}
	} else if err == nil && _err != nil {
		err = _err
	} else if err == nil && _err != nil {
		err = _err
	}
	if zi.pcmd == "" {
		if ppst.Comm != "" {
			zi.pcmd = ppst.Comm
		}
	}
	if zi.pcmd != "" {
		(*pidCache)[pidT(ppst.PPID)] = zi.pcmd
	}
	return zi, err
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
			if err != nil {
				log.Printf("Failed to get processes: %v\n", err)
				continue
			}
			evalFailed := 0

			var zombieCounter = map[zombieInfo]uint{}
			var pidCache = make(pid2cmdT)

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
				var zi zombieInfo
				zi, err = getZombieInfo(&p, &pst, &pidCache)
				if err != nil {
					evalFailed++
					continue procsLoop
				}
				zombieCounter[zi]++
			}

			for zi, count := range zombieCounter {
				opsZombies.With(prometheus.Labels{
					"cmd":  zi.cmd,
					"pcmd": zi.pcmd,
				}).Set(float64(count))
			}

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
	}, []string{"cmd", "pcmd"})
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
