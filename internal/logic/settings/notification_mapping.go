package settings

import (
	"strings"

	"github.com/relayx-tech/panel/internal/logic/shared"
	"github.com/relayx-tech/panel/internal/model"
	"github.com/relayx-tech/panel/internal/types"
)

func toNotificationChannel(c model.NotificationChannel) types.NotificationChannel {
	return types.NotificationChannel{
		Id: c.ID, Name: c.Name, ChannelType: c.Type, TargetMasked: c.TargetMasked,
		MinLevel: c.MinLevel, Enabled: c.Enabled, LastSentAt: shared.TimeStringOpt(c.LastSentAt),
	}
}

func validLevel(level string) bool {
	switch strings.ToLower(level) {
	case "info", "warning", "critical":
		return true
	default:
		return false
	}
}

func validChannelType(kind string) bool {
	switch strings.ToLower(kind) {
	case "telegram", "webhook":
		return true
	default:
		return false
	}
}
