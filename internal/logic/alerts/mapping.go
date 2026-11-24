package alerts

import (
	"github.com/relayx-tech/panel/internal/logic/shared"
	"github.com/relayx-tech/panel/internal/model"
	"github.com/relayx-tech/panel/internal/types"
)

func toAlert(a model.Alert) types.Alert {
	out := types.Alert{
		Id: a.ID, Level: a.Level, AlertType: a.Type, ObjectType: a.ObjectType,
		ObjectId: a.ObjectID, ObjectName: a.ObjectName, Message: a.Message, Status: a.Status,
		FirstSeenAt: shared.TimeString(a.FirstSeenAt), LastSeenAt: shared.TimeString(a.LastSeenAt),
		ResolvedAt: shared.TimeStringOpt(a.ResolvedAt),
	}
	if a.Notification != nil {
		out.Notification = &types.AlertNotificationInfo{
			Status: a.Notification.Status, Channel: a.Notification.ErrorCode,
			ErrorCode: a.Notification.ErrorCode, SentAt: shared.TimeString(a.Notification.SentAt),
		}
	}
	return out
}
