// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package nodes

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/protobuf/types/known/timestamppb"

	agentv1 "github.com/relayx-tech/contracts/gen/go/agent/v1"
	"github.com/relayx-tech/panel/internal/errcode"
	"github.com/relayx-tech/panel/internal/model"
	"github.com/relayx-tech/panel/internal/requestid"
	"github.com/relayx-tech/panel/internal/response"
	"github.com/relayx-tech/panel/internal/svc"
	"github.com/relayx-tech/panel/internal/types"
)

const (
	defaultProbeTimeoutMs = 5000
	maxProbeTimeoutMs     = 30000
)

type NodeProbeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 立即拨测
func NewNodeProbeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *NodeProbeLogic {
	return &NodeProbeLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *NodeProbeLogic) NodeProbe(req *types.NodeProbeReq) (resp *types.NodeProbeResp, err error) {
	probeType := strings.ToLower(strings.TrimSpace(req.ProbeType))
	if probeType != "tcp" && probeType != "http" {
		return nil, response.NewFieldError(errcode.VALIDATION_INVALID_FIELD, "type", "type must be tcp or http")
	}
	if strings.TrimSpace(req.Target) == "" {
		return nil, response.MissingField("target")
	}
	timeout := req.TimeoutMs
	if timeout <= 0 {
		timeout = defaultProbeTimeoutMs
	}
	if timeout > maxProbeTimeoutMs {
		return nil, response.NewFieldError(errcode.VALIDATION_INVALID_FIELD, "timeoutMs", "timeoutMs must be <= 30000")
	}
	if _, err := l.svcCtx.NodeModel.Get(l.ctx, req.Id); err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, response.NewError(errcode.NODE_NOT_FOUND, "node not found")
		}
		return nil, response.Internal(err)
	}
	if l.svcCtx.Agent == nil {
		return nil, response.NewError(errcode.UPSTREAM_AGENT_UNAVAILABLE, "agent is not configured")
	}

	pbType := agentv1.ProbeType_PROBE_TYPE_TCP
	if probeType == "http" {
		pbType = agentv1.ProbeType_PROBE_TYPE_HTTP
	}
	expected := ""
	if req.ExpectedStatus > 0 {
		expected = strconv.Itoa(req.ExpectedStatus)
	}
	now := time.Now().UTC()
	ctx, cancel := context.WithTimeout(l.ctx, time.Duration(timeout)*time.Millisecond+time.Second)
	defer cancel()
	probeResp, err := l.svcCtx.Agent.Probe(ctx, &agentv1.ProbeRequest{
		Meta: &agentv1.AgentRequestMeta{
			RequestId: requestid.FromContext(l.ctx),
			Timestamp: timestamppb.New(now),
		},
		NodeId:         req.Id,
		Type:           pbType,
		Target:         req.Target,
		TimeoutMs:      uint32(timeout),
		ExpectedStatus: expected,
	})
	if err != nil {
		l.Errorw("node.probe.failure", logx.Field("nodeId", req.Id), logx.Field("error", err.Error()))
		return nil, response.NewError(errcode.UPSTREAM_AGENT_UNAVAILABLE, "agent is unreachable")
	}
	return &types.NodeProbeResp{
		Code: errcode.OK,
		Data: types.NodeProbeResult{
			Success: probeResp.GetSuccess(), LatencyMs: int64(probeResp.GetLatencyMs()),
			StatusCode: int(probeResp.GetStatusCode()), Error: probeResp.GetError(),
		},
		RequestId: requestid.FromContext(l.ctx),
	}, nil
}
