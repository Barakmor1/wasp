package shortage_detector

import (
	"kubevirt.io/wasp/pkg/log"
	stats_collector "kubevirt.io/wasp/pkg/wasp/stats-collector"
	"time"
)

// ShortageDetector is an interface for shortage detection
type ShortageDetector interface {
	ShouldEvict() bool
}

type ShortageDetectorImpl struct {
	sc                      stats_collector.StatsCollector
	maxSwapInRate           float32
	maxSwapOutRate          float32
	minAvailableMemoryBytes int64
	minTimeInterval         time.Duration
}

func NewShortageDetectorImpl(sc stats_collector.StatsCollector, maxSwapInRate, maxSwapOutRate float32, minAvailableMemoryBytes int64, minTimeInterval time.Duration) *ShortageDetectorImpl {
	return &ShortageDetectorImpl{
		sc:                      sc,
		maxSwapInRate:           maxSwapInRate,
		maxSwapOutRate:          maxSwapOutRate,
		minAvailableMemoryBytes: minAvailableMemoryBytes,
		minTimeInterval:         minTimeInterval,
	}
}

func (sdi *ShortageDetectorImpl) ShouldEvict() bool {
	stats := sdi.sc.GetStatsList()
	if len(stats) < 2 {
		log.Log.Infof("not enough stats provided, need at least 2")
		return false
	}

	// Find the second newest Stats object after the first one with at least minTimeInterval difference
	firstStat := stats[0]
	var secondNewest *stats_collector.Stats
	for i := 1; i < len(stats); i++ {
		if firstStat.Time.Sub(stats[i].Time) >= sdi.minTimeInterval {
			secondNewest = &stats[i]
			break
		}
	}

	if secondNewest == nil {
		log.Log.Infof("could not find second newest Stats with at least %v difference", sdi.minTimeInterval)
		return false
	}

	// Calculate time difference in seconds
	timeDiffSeconds := float32(firstStat.Time.Sub(secondNewest.Time).Seconds())

	// Calculate rates
	swapInRate := float32(firstStat.SwapIn-secondNewest.SwapIn) / timeDiffSeconds
	swapOutRate := float32(firstStat.SwapOut-secondNewest.SwapOut) / timeDiffSeconds

	// Check conditions
	if swapInRate > sdi.maxSwapInRate && swapOutRate > sdi.maxSwapOutRate &&
		firstStat.AvailableMemoryBytes < uint64(sdi.minAvailableMemoryBytes) {
		return true
	}

	return false
}
