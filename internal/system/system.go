// Package system 提供面板构建信息（-ldflags 注入）。
package system

import "time"

var (
	Version   = "dev"
	Commit    = ""
	BuildTime = ""
	StartedAt = time.Now()
)

const (
	// ContractVersion 是 Panel 支持的 Agent 合同版本（STO-004 minor）。
	ContractVersion = "1.4.0"
	// ContractMajor 是合同 major 版本，用于兼容矩阵。
	ContractMajor = 1
	// SchemaVersion 是本 Panel 的配置 schema 版本。
	SchemaVersion = "1"
	// CoreControlVersion / CoreNodeMinMax / XrayVersion 未上报时展示「—」。
	CoreControlVersion = ""
	CoreNodeMin        = ""
	CoreNodeMax        = ""
	XrayBaseline       = ""
)
