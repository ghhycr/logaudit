package main

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// ==================== 宿主机监控（总览仪表盘 · 主机硬件信息） ====================
// GET /api/v1/host/overview（requireAuth）
// 数据来源：容器挂载宿主机 /proc（只读）与 /（只读），Redis 缓存 10s
// 部署要求（docker-compose api 服务）：
//   volumes:
//     - /proc:/hostproc:ro
//     - /:/hostroot:ro

const HOSTPROC = "/hostproc"
const HOSTROOT = "/hostroot"

type CPUInfo struct {
	Cores    int     `json:"cores"`
	UsagePct float64 `json:"usage_percent"`
}

type MemInfo struct {
	TotalGB  float64 `json:"total_gb"`
	UsedGB   float64 `json:"used_gb"`
	UsagePct float64 `json:"usage_percent"`
}

type DiskInfo struct {
	TotalGB  float64 `json:"total_gb"`
	UsedGB   float64 `json:"used_gb"`
	UsagePct float64 `json:"usage_percent"`
	Mount    string  `json:"mount"` // 展示用：数据盘 /data 或 根分区 /
}

type HostOverview struct {
	Hostname    string   `json:"hostname"`
	UptimeSec   int64    `json:"uptime_seconds"`
	CPU         CPUInfo  `json:"cpu"`
	Memory      MemInfo  `json:"memory"`
	Disk        DiskInfo `json:"disk"`
	CollectedAt string   `json:"collected_at"`
}

func readCPUJiffies() (idle, total uint64, ok bool) {
	b, err := os.ReadFile(HOSTPROC + "/stat")
	if err != nil {
		return 0, 0, false
	}
	line := ""
	for _, l := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(l, "cpu ") {
			line = l
			break
		}
	}
	if line == "" {
		return 0, 0, false
	}
	fields := strings.Fields(line)[1:] // user nice system idle iowait irq softirq steal
	if len(fields) < 4 {
		return 0, 0, false
	}
	var vals [8]uint64
	for i := 0; i < len(fields) && i < 8; i++ {
		vals[i], _ = strconv.ParseUint(fields[i], 10, 64)
	}
	idle = vals[3] + vals[4] // idle + iowait
	for _, v := range vals {
		total += v
	}
	return idle, total, true
}

func readMemInfo() (totalKB, availKB uint64) {
	b, err := os.ReadFile(HOSTPROC + "/meminfo")
	if err != nil {
		return 0, 0
	}
	for _, l := range strings.Split(string(b), "\n") {
		f := strings.Fields(l)
		if len(f) < 2 {
			continue
		}
		switch f[0] {
		case "MemTotal:":
			totalKB, _ = strconv.ParseUint(f[1], 10, 64)
		case "MemAvailable:":
			availKB, _ = strconv.ParseUint(f[1], 10, 64)
		}
	}
	return totalKB, availKB
}

func readUptime() int64 {
	// 宿主机视角（/hostroot 为宿主机根；/hostproc/uptime 是容器 namespace 的 uptime）
	b, err := os.ReadFile(HOSTROOT + "/proc/uptime")
	if err != nil {
		b, err = os.ReadFile(HOSTPROC + "/uptime")
		if err != nil {
			return 0
		}
	}
	f := strings.Fields(string(b))
	if len(f) < 1 {
		return 0
	}
	sec, _ := strconv.ParseFloat(f[0], 64)
	return int64(sec)
}

func readHostname() string {
	// 优先读取宿主机 /etc/hostname（/ 已只读挂载到 /hostroot；/proc 内的 hostname 是容器视角）
	if b, err := os.ReadFile(HOSTROOT + "/etc/hostname"); err == nil {
		if h := strings.TrimSpace(string(b)); h != "" {
			return h
		}
	}
	if h, err := os.Hostname(); err == nil {
		return h
	}
	return "unknown"
}

func gbFromKB(kb uint64) float64 {
	return float64(kb) / 1024 / 1024
}

func (s *Server) handleHostOverview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// Redis 缓存 10s（监控数据不要求实时）
	if s.rdb != nil {
		if v, err := s.rdb.Get(ctx, "stats:host").Result(); err == nil && v != "" {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			_, _ = w.Write([]byte(v))
			return
		}
	}

	// CPU：两次采样计算使用率（间隔 200ms）
	var cpu CPUInfo
	if b, err := os.ReadFile(HOSTPROC + "/cpuinfo"); err == nil {
		cpu.Cores = strings.Count(string(b), "processor\t:")
		if cpu.Cores == 0 {
			cpu.Cores = strings.Count(string(b), "processor :")
		}
	}
	idle0, total0, ok0 := readCPUJiffies()
	time.Sleep(200 * time.Millisecond)
	idle1, total1, ok1 := readCPUJiffies()
	if ok0 && ok1 && total1 > total0 {
		idleD := idle1 - idle0
		totalD := total1 - total0
		cpu.UsagePct = round2(100 * (1 - float64(idleD)/float64(totalD)))
	}
	if cpu.Cores <= 0 {
		cpu.Cores = 1
	}

	// 内存
	var mem MemInfo
	totalKB, availKB := readMemInfo()
	if totalKB > 0 {
		mem.TotalGB = gbFromKB(totalKB)
		mem.UsedGB = round2(gbFromKB(totalKB - availKB))
		mem.UsagePct = round2(100 * float64(totalKB-availKB) / float64(totalKB))
	}

	// 磁盘：优先日志数据盘 /data（sdb，宿主机根 / 只读挂载在 /hostroot），不存在时回退根分区
	var disk DiskInfo
	var st syscall.Statfs_t
	diskPath := HOSTROOT + "/data"
	if err := syscall.Statfs(diskPath, &st); err != nil || st.Blocks == 0 {
		diskPath = HOSTROOT
		if err := syscall.Statfs(diskPath, &st); err != nil {
			st.Blocks = 0
		}
	}
	if diskPath == HOSTROOT+"/data" {
		disk.Mount = "数据盘 /data"
	} else {
		disk.Mount = "根分区 /"
	}
	if st.Blocks > 0 {
		totalB := st.Blocks * uint64(st.Bsize)
		freeB := st.Bavail * uint64(st.Bsize)
		usedB := totalB - freeB
		disk.TotalGB = round2(float64(totalB) / 1024 / 1024 / 1024)
		disk.UsedGB = round2(float64(usedB) / 1024 / 1024 / 1024)
		disk.UsagePct = round2(100 * float64(usedB) / float64(totalB))
	}

	out := HostOverview{
		Hostname:    readHostname(),
		UptimeSec:   readUptime(),
		CPU:         cpu,
		Memory:      mem,
		Disk:        disk,
		CollectedAt: time.Now().Format("2006-01-02 15:04:05"),
	}
	if s.rdb != nil {
		if b, err := json.Marshal(map[string]any{"code": 0, "message": "ok", "data": out}); err == nil {
			s.rdb.Set(ctx, "stats:host", b, 10*time.Second)
		}
	}
	ok(w, out)
}

func round2(v float64) float64 {
	return float64(int64(v*100+0.5)) / 100
}
