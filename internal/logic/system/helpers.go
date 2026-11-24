package system

import (
	"time"

	panelsystem "github.com/relayx-tech/panel/internal/system"
)

func dashIfEmpty(v string) string {
	if v == "" {
		return "—"
	}
	return v
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func timeSinceStart() time.Duration {
	if panelsystem.StartedAt.IsZero() {
		return 0
	}
	return time.Since(panelsystem.StartedAt)
}
