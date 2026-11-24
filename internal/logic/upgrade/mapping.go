package upgrade

import (
	"context"
	"errors"
	"time"

	"github.com/relayx-tech/panel/internal/auth"
	"github.com/relayx-tech/panel/internal/errcode"
	"github.com/relayx-tech/panel/internal/model"
	"github.com/relayx-tech/panel/internal/response"
	"github.com/relayx-tech/panel/internal/types"
	"github.com/relayx-tech/panel/internal/upgrade"
)

func mapError(err error) *response.AppError {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, upgrade.ErrLockBusy):
		return response.NewError(errcode.OPERATION_IN_PROGRESS, "another upgrade is in progress")
	case errors.Is(err, upgrade.ErrComposeManual):
		return response.NewError(errcode.CONFLICT_COMPOSE_MANUAL_REQUIRED, "docker compose pull && docker compose up -d")
	case errors.Is(err, upgrade.ErrUpgraderUnavailable):
		return response.NewError(errcode.UPSTREAM_UPGRADER_UNAVAILABLE, "upgrader is unavailable")
	case errors.Is(err, upgrade.ErrAgentUnavailable):
		return response.NewError(errcode.UPSTREAM_AGENT_UNAVAILABLE, "agent is unavailable")
	case errors.Is(err, upgrade.ErrDowngrade), errors.Is(err, upgrade.ErrSameVersion), errors.Is(err, upgrade.ErrMajorMismatch):
		return response.NewError(errcode.CONFLICT_UPGRADE_TARGET_INVALID, err.Error())
	case errors.Is(err, upgrade.ErrContractMajor), errors.Is(err, upgrade.ErrAgentMinorAhead),
		errors.Is(err, upgrade.ErrSchemaMismatch), errors.Is(err, upgrade.ErrInvalidVersion):
		return response.NewError(errcode.VERSION_INCOMPATIBLE, err.Error())
	case errors.Is(err, upgrade.ErrTaskNotRetryable):
		return response.NewError(errcode.CONFLICT_UPGRADE_TARGET_INVALID, "task is not retryable")
	case errors.Is(err, model.ErrNotFound):
		return response.NewError(errcode.NOT_FOUND_UPGRADE_TASK, "upgrade task not found")
	default:
		return response.Internal(err)
	}
}

func toTaskNode(n model.UpgradeTaskNode) types.UpgradeTaskNode {
	return types.UpgradeTaskNode{
		NodeId: n.NodeID, NodeName: n.NodeName, FromVersion: n.FromVersion, ToVersion: n.ToVersion,
		State: n.State, ErrorCode: n.ErrorCode, ErrorMessage: n.ErrorMessage,
		UpdatedAt: n.UpdatedAt.UTC().Format("2006-01-02T15:04:05.000Z07:00"),
	}
}

func toTask(t model.UpgradeTask) types.UpgradeTask {
	out := types.UpgradeTask{
		Id: t.ID, Scope: t.Scope, TargetVersion: t.TargetVersion, FromVersion: t.FromVersion,
		BatchId: t.BatchID, Status: t.Status, ErrorCode: t.ErrorCode, ErrorMessage: t.ErrorMessage,
		CreatedBy: t.CreatedBy, RequestId: t.RequestID,
		CreatedAt: t.CreatedAt.UTC().Format("2006-01-02T15:04:05.000Z07:00"),
	}
	if t.StartedAt != nil {
		out.StartedAt = t.StartedAt.UTC().Format("2006-01-02T15:04:05.000Z07:00")
	}
	if t.FinishedAt != nil {
		out.FinishedAt = t.FinishedAt.UTC().Format("2006-01-02T15:04:05.000Z07:00")
	}
	if t.Nodes != nil {
		out.Nodes = make([]types.UpgradeTaskNode, 0, len(t.Nodes))
		for _, n := range t.Nodes {
			out.Nodes = append(out.Nodes, toTaskNode(n))
		}
	}
	return out
}

func compatibilityInfo() types.CompatibilityInfo {
	return types.CompatibilityInfo{
		ContractMajor: upgrade.ContractMajor, SchemaVersion: "1",
		AgentMinVersion: "", AgentMaxVersion: "", Compatible: true,
	}
}

func nowUTC() time.Time { return time.Now().UTC() }

func actorFrom(ctx context.Context) string {
	if row := auth.SessionFromContext(ctx); row != nil {
		return row.User.Account
	}
	return ""
}
