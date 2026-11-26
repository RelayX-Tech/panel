package nodes

import (
	"context"
	"time"

	"github.com/relayx-tech/panel/internal/logic/shared"
	"github.com/relayx-tech/panel/internal/model"
	"github.com/relayx-tech/panel/internal/types"
)

// displayStates 由升级任务派生节点展示态（design.md §6.3）：不写回 nodes.status。
func (l *NodeListLogic) displayStates(ctx context.Context) map[string]string {
	states := make(map[string]string)
	if l.svcCtx.UpgradeModel == nil {
		return states
	}
	if task, err := l.svcCtx.UpgradeModel.ActiveTask(ctx); err == nil && task != nil {
		switch task.Status {
		case model.UpgradeTaskRunning, model.UpgradeTaskRollingBack:
			for _, n := range task.Nodes {
				switch n.State {
				case model.NodeUpgradePending, model.NodeUpgradeDownloading, model.NodeUpgradeVerifying, model.NodeUpgradeApplying:
					states[n.NodeID] = "upgrading"
				}
			}
		}
	}
	if targets, err := l.svcCtx.UpgradeModel.ListTargets(ctx); err == nil {
		for _, t := range targets {
			if t.ExpiresAt.After(time.Now()) {
				if _, taken := states[t.NodeID]; !taken {
					states[t.NodeID] = "pending_upgrade"
				}
			}
		}
	}
	return states
}

func toNodeSummary(n model.NodeSummary, displayState string) types.NodeSummary {
	return types.NodeSummary{
		Id:             n.ID,
		Name:           n.Name,
		NodeType:       n.Type,
		GroupId:        n.GroupID,
		GroupName:      n.GroupName,
		Region:         n.Region,
		Ip:             n.IP,
		Status:         n.Status,
		DisplayState:   displayState,
		CpuPercent:     n.CPUPercent,
		MemPercent:     n.MemPercent,
		DiskPercent:    n.DiskPercent,
		Load1:          n.Load1,
		TcpConnections: n.TCPConnections,
		NetInBps:       n.NetInBps,
		NetOutBps:      n.NetOutBps,
		XrayState:      n.XrayState,
		Restarts:       n.Restarts,
		LastSeenAt:     shared.TimeStringOpt(n.LastSeenAt),
	}
}

func toNodeDetail(d *model.NodeDetail, displayState string) types.NodeDetail {
	nics := make([]types.NodeNIC, 0, len(d.NICs))
	for _, nic := range d.NICs {
		nics = append(nics, types.NodeNIC{Name: nic.Name, Mac: nic.MAC, Mtu: nic.MTU, Up: nic.Up, Addresses: nic.Addresses})
	}
	return types.NodeDetail{
		Id:                  d.ID,
		Name:                d.Name,
		NodeType:            d.Type,
		GroupId:             d.GroupID,
		GroupName:           d.GroupName,
		Region:              d.Region,
		Ip:                  d.IP,
		Status:              d.Status,
		DisplayState:        displayState,
		CpuPercent:          d.CPUPercent,
		MemPercent:          d.MemPercent,
		DiskPercent:         d.DiskPercent,
		Load1:               d.Load1,
		TcpConnections:      d.TCPConnections,
		NetInBps:            d.NetInBps,
		NetOutBps:           d.NetOutBps,
		XrayState:           d.XrayState,
		Restarts:            d.Restarts,
		LastSeenAt:          shared.TimeStringOpt(d.LastSeenAt),
		AgentVersion:        d.AgentVersion,
		XrayVersion:         d.XrayVersion,
		CertificateNotAfter: shared.TimeStringOpt(d.CertificateNotAfter),
		Nics:                nics,
		Xray: types.NodeXray{
			Pid: d.Xray.PID, CpuPercent: d.Xray.CPUPercent, MemBytes: d.Xray.MemBytes,
			Restarts: d.Xray.Restarts, Inbounds: d.Xray.Inbounds, Outbounds: d.Xray.Outbounds,
		},
		Probe: types.NodeProbeInfo{
			ProbeType: d.Probe.Type, LatencyMs: d.Probe.LatencyMs, Success: d.Probe.Success,
			Failures: d.Probe.Failures, ConsecutiveFailures: d.Probe.ConsecutiveFailures,
			CheckedAt: shared.TimeStringOpt(d.Probe.CheckedAt),
		},
	}
}
