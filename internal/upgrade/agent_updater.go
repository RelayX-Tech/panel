package upgrade

import (
	"context"
	"time"

	agentv1 "github.com/relayx-tech/contracts/gen/go/agent/v1"
	"google.golang.org/protobuf/types/known/timestamppb"

	agentclient "github.com/relayx-tech/panel/internal/client/agent"
)

// GRPCAgentUpdater 通过 AgentControlService.Update 下发升级并读取状态。
type GRPCAgentUpdater struct {
	Client *agentclient.Client
}

var _ AgentUpdater = (*GRPCAgentUpdater)(nil)

func (u *GRPCAgentUpdater) Update(ctx context.Context, req AgentUpdateRequest) (AgentUpdateResult, error) {
	deadline := req.Deadline
	if deadline <= 0 {
		deadline = 10 * time.Minute
	}
	resp, err := u.Client.Update(ctx, &agentv1.AgentUpdateRequest{
		Meta: &agentv1.AgentRequestMeta{
			RequestId:   req.OperationID,
			OperationId: req.OperationID,
			Timestamp:   timestamppb.New(time.Now().UTC()),
		},
		NodeId:          req.NodeID,
		OperationId:     req.OperationID,
		TargetVersion:   req.TargetVersion,
		SourceUrl:       req.SourceURL,
		Sha256:          req.SHA256,
		Signature:       req.Signature,
		DeadlineSeconds: uint32(deadline.Seconds()),
		Reason:          req.Reason,
	})
	if err != nil {
		return AgentUpdateResult{}, err
	}
	result := AgentUpdateResult{
		Accepted: resp.GetAccepted(), State: resp.GetState(), CurrentVersion: resp.GetCurrentVersion(),
	}
	if e := resp.GetError(); e != nil {
		result.ErrorCode = e.GetCode()
	}
	return result, nil
}

func (u *GRPCAgentUpdater) Status(ctx context.Context, nodeID string) (AgentUpdateStatus, error) {
	resp, err := u.Client.Status(ctx, &agentv1.StatusRequest{NodeId: nodeID})
	if err != nil {
		return AgentUpdateStatus{}, err
	}
	state := resp.GetUpdateState()
	return AgentUpdateStatus{
		State:         state.GetState(),
		TargetVersion: state.GetTargetVersion(),
		ErrorCode:     state.GetErrorCode(),
		ErrorMessage:  state.GetErrorMessage(),
	}, nil
}
