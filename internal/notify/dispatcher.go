package notify

import (
	"context"
	"time"

	"github.com/relayx-tech/panel/internal/model"
)

// Decrypter 解密通知目标（生产实现为 cryptobox.Box）。
type Decrypter interface {
	Open([]byte) ([]byte, error)
}

// ChannelSource 提供通知渠道与投递记录写入。
type ChannelSource interface {
	List(ctx context.Context) ([]model.NotificationChannel, error)
	MarkSent(ctx context.Context, id int64, at time.Time) error
}

// AlertRecorder 写入告警投递记录。
type AlertRecorder interface {
	InsertNotification(ctx context.Context, n model.AlertNotification) error
}

// Dispatcher 按级别把告警投递到已启用渠道（design.md §5.4 / AC120）。
type Dispatcher struct {
	Channels ChannelSource
	Alerts   AlertRecorder
	Box      Decrypter
	Sender   *Client
	Now      func() time.Time
}

func (d *Dispatcher) Dispatch(ctx context.Context, alertID int64, level, message string) {
	if d.Channels == nil || d.Box == nil {
		return
	}
	channels, err := d.Channels.List(ctx)
	if err != nil {
		return
	}
	now := d.Now
	if now == nil {
		now = time.Now
	}
	for _, channel := range channels {
		if !channel.Enabled || levelRank(channel.MinLevel) > levelRank(level) {
			continue
		}
		status, code := "sent", ""
		raw, derr := d.Box.Open(channel.TargetEncrypted)
		if derr != nil {
			status, code = "failed", "UPSTREAM_NOTIFICATION_FAILED"
		} else if serr := d.Sender.Send(ctx, channel.Type, string(raw), message); serr != nil {
			status, code = "failed", "UPSTREAM_NOTIFICATION_FAILED"
		} else {
			_ = d.Channels.MarkSent(ctx, channel.ID, now().UTC())
		}
		if d.Alerts != nil {
			_ = d.Alerts.InsertNotification(ctx, model.AlertNotification{
				AlertID: alertID, ChannelID: channel.ID, Status: status, ErrorCode: code, SentAt: now().UTC(),
			})
		}
	}
}

func levelRank(level string) int {
	switch level {
	case "info":
		return 0
	case "warning":
		return 1
	case "critical":
		return 2
	default:
		return 1
	}
}
