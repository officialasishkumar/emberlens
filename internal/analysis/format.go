package analysis

import (
	"fmt"
	"sort"
	"time"
)

func formatCount(v int) string {
	return fmt.Sprintf("%d", v)
}

func formatDuration(d time.Duration, unit string) string {
	if d < 0 {
		d = 0
	}

	switch unit {
	case "hours", "hour", "h":
		return fmt.Sprintf("%.1fh", d.Hours())
	default:
		return fmt.Sprintf("%.1fd", d.Hours()/24)
	}
}

func percentileDuration(values []time.Duration, percentile float64) time.Duration {
	if len(values) == 0 {
		return 0
	}
	cp := append([]time.Duration(nil), values...)
	sort.Slice(cp, func(i, j int) bool { return cp[i] < cp[j] })
	index := int(float64(len(cp)-1) * percentile)
	if index < 0 {
		index = 0
	}
	if index >= len(cp) {
		index = len(cp) - 1
	}
	return cp[index]
}
