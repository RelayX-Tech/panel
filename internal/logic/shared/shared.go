// Package shared 提供 STO-004 各 logic 共用的时间格式、范围解析与视图映射。
package shared

import (
	"time"
)

// TimeString 输出 UTC RFC3339 毫秒（design.md §3.1）。
func TimeString(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05.000Z07:00")
}

// TimeStringOpt 对可空时间输出空串（JSON 中保持字段存在）。
func TimeStringOpt(t *time.Time) string {
	if t == nil {
		return ""
	}
	return TimeString(*t)
}

// RangeWindow 把 range 枚举映射为时间窗口与聚合步长；非法值返回 false。
func RangeWindow(value string, now time.Time) (from, to time.Time, step time.Duration, ok bool) {
	now = now.UTC()
	switch value {
	case "", "24h":
		return now.Add(-24 * time.Hour), now, 15 * time.Minute, true
	case "1h":
		return now.Add(-time.Hour), now, time.Minute, true
	case "6h":
		return now.Add(-6 * time.Hour), now, 5 * time.Minute, true
	case "7d":
		return now.Add(-7 * 24 * time.Hour), now, time.Hour, true
	case "30d":
		return now.Add(-30 * 24 * time.Hour), now, 6 * time.Hour, true
	default:
		return time.Time{}, time.Time{}, 0, false
	}
}

// RangeOrDefault 返回合法 range（缺省 24h）。
func RangeOrDefault(value string) string {
	switch value {
	case "1h", "6h", "24h", "7d", "30d":
		return value
	default:
		return "24h"
	}
}
