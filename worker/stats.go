package worker

import (
	"log"

	"github.com/c9s/goprocinfo/linux"
)

type Stats struct {
	MemStats  *linux.MemInfo
	DiskStats *linux.Disk
	CpuStats  *linux.CPUStat
	LoadStats *linux.LoadAvg
	TaskCount int
}

func GetStats(l *log.Logger) *Stats {
	return &Stats{
		MemStats:  GetMemoryInfo(l),
		DiskStats: GetDiskInfo(l),
		CpuStats:  GetCpuStats(l),
		LoadStats: GetLoadAvg(l),
	}
}

func (s *Stats) MemTotalKb() uint64 {
	return s.MemStats.MemTotal
}

func (s *Stats) MemAvailableKb() uint64 {
	return s.MemStats.MemAvailable
}

func (s *Stats) MemUsedKb() uint64 {
	return s.MemStats.MemTotal - s.MemStats.MemAvailable
}

func (s *Stats) MemUsedPercent() uint64 {
	return s.MemStats.MemAvailable / s.MemStats.MemTotal
}

func (s *Stats) DiskTotal() uint64 {
	return s.DiskStats.All
}

func (s *Stats) DiskFree() uint64 {
	return s.DiskStats.Free
}

func (s *Stats) DiskUsed() uint64 {
	return s.DiskStats.Used
}

func (s *Stats) CpuUsage() float64 {
	cpuStats := s.CpuStats

	idle := cpuStats.Idle + cpuStats.IOWait
	nonIdle := cpuStats.User + cpuStats.Nice +
		cpuStats.System + cpuStats.IRQ + cpuStats.SoftIRQ +
		cpuStats.Steal

	total := idle + nonIdle
	if total == 0 {
		return 0.00
	}
	return (float64(total) - float64(idle)) / float64(total)
}

func GetMemoryInfo(log *log.Logger) *linux.MemInfo {
	memStats, err := linux.ReadMemInfo("/proc/meminfo")
	if err != nil {
		log.Printf("error reading from /prod/meminfo: %v\n", err)
		return &linux.MemInfo{}
	}

	return memStats
}

func GetDiskInfo(log *log.Logger) *linux.Disk {
	diskStats, err := linux.ReadDisk("/")
	if err != nil {
		log.Printf("error reading from / : %v\n", err)
		return &linux.Disk{}
	}
	return diskStats
}

func GetCpuStats(log *log.Logger) *linux.CPUStat {
	stats, err := linux.ReadStat("/proc/stat")
	if err != nil {
		log.Printf("error reading from /proc/stat: %v", err)
		return &linux.CPUStat{}
	}
	return &stats.CPUStatAll
}

func GetLoadAvg(log *log.Logger) *linux.LoadAvg {
	loadAvg, err := linux.ReadLoadAvg("/proc/loadavg")
	if err != nil {
		log.Printf("error reading from /proc/loadavg")
		return &linux.LoadAvg{}
	}

	return loadAvg
}
