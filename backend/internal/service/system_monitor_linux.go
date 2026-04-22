//go:build linux

package service

import (
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func getDiskInfo() (DiskInfo, error) {
	workspaceRoot := getWorkspaceRoot()

	var stat syscall.Statfs_t
	err := syscall.Statfs(workspaceRoot, &stat)
	if err != nil {
		return DiskInfo{}, err
	}

	totalBytes := stat.Blocks * uint64(stat.Bsize)
	freeBytes := stat.Bfree * uint64(stat.Bsize)
	usedPct := 0.0
	if totalBytes > 0 {
		usedPct = float64(totalBytes-freeBytes) / float64(totalBytes) * 100
	}

	return DiskInfo{
		TotalBytes:     totalBytes,
		FreeBytes:      freeBytes,
		UsedPercentage: usedPct,
	}, nil
}

func getMemoryInfo() (*memoryInfo, error) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return nil, err
	}

	memInfo := map[string]uint64{}
	for _, line := range strings.Split(string(data), "\n") {
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		key := strings.TrimSuffix(parts[0], ":")
		val, err := strconv.ParseUint(parts[1], 10, 64)
		if err != nil {
			continue
		}
		memInfo[key] = val * 1024
	}

	total := memInfo["MemTotal"]
	avail := memInfo["MemAvailable"]
	if total == 0 {
		total = memInfo["MemFree"] + memInfo["Buffers"] + memInfo["Cached"]
	}
	used := total - avail
	pct := 0.0
	if total > 0 {
		pct = float64(used) / float64(total) * 100
	}

	return &memoryInfo{
		TotalBytes: total,
		UsedBytes:  used,
		Percentage: pct,
	}, nil
}

func getCPUPercentage() float64 {
	readStat := func() ([]uint64, error) {
		data, err := os.ReadFile("/proc/stat")
		if err != nil {
			return nil, err
		}
		for _, line := range strings.Split(string(data), "\n") {
			if !strings.HasPrefix(line, "cpu ") {
				continue
			}
			fields := strings.Fields(line)
			values := make([]uint64, len(fields)-1)
			for i, f := range fields[1:] {
				v, err := strconv.ParseUint(f, 10, 64)
				if err != nil {
					v = 0
				}
				values[i] = v
			}
			return values, nil
		}
		return nil, os.ErrNotExist
	}

	s1, err := readStat()
	if err != nil {
		return 0
	}

	time.Sleep(500 * time.Millisecond)

	s2, err := readStat()
	if err != nil {
		return 0
	}

	// fields: user, nice, system, idle, iowait, irq, softirq, steal
	idle1 := s1[3] + s1[4]
	idle2 := s2[3] + s2[4]

	total1 := uint64(0)
	total2 := uint64(0)
	for _, v := range s1 {
		total1 += v
	}
	for _, v := range s2 {
		total2 += v
	}

	idleDelta := idle2 - idle1
	totalDelta := total2 - total1
	if totalDelta == 0 {
		return 0
	}
	return float64(totalDelta-idleDelta) / float64(totalDelta) * 100
}
