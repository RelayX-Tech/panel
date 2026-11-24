package upgrade

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var (
	ErrInvalidVersion   = errors.New("invalid version")
	ErrDowngrade        = errors.New("downgrade is not allowed")
	ErrMajorMismatch    = errors.New("major version mismatch")
	ErrSameVersion      = errors.New("target version equals current version")
	ErrContractMajor    = errors.New("contract major version mismatch")
	ErrAgentMinorAhead  = errors.New("agent minor version is ahead of panel")
	ErrSchemaMismatch   = errors.New("schema version mismatch")
	ErrCompatibilityBad = errors.New("version is not compatible")
)

// ContractMajor 是 Panel 支持的 Agent 合同 major 版本。
const ContractMajor = 1

// Version 是简单的 major.minor.patch 版本（design.md §7.1 / §7.2）。
type Version struct {
	Major int
	Minor int
	Patch int
}

// ParseVersion 解析 "x.y.z"；允许缺省 patch。
func ParseVersion(raw string) (Version, error) {
	parts := strings.Split(strings.TrimSpace(raw), ".")
	if len(parts) < 2 || len(parts) > 3 {
		return Version{}, fmt.Errorf("%w: %q", ErrInvalidVersion, raw)
	}
	nums := make([]int, 3)
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return Version{}, fmt.Errorf("%w: %q", ErrInvalidVersion, raw)
		}
		nums[i] = n
	}
	return Version{Major: nums[0], Minor: nums[1], Patch: nums[2]}, nil
}

// Compare 返回 -1 / 0 / 1。
func (v Version) Compare(o Version) int {
	for _, pair := range [][2]int{{v.Major, o.Major}, {v.Minor, o.Minor}, {v.Patch, o.Patch}} {
		if pair[0] < pair[1] {
			return -1
		}
		if pair[0] > pair[1] {
			return 1
		}
	}
	return 0
}

// CheckTarget 校验升级目标：同 major、严格更高版本（禁止降级与同版本）。
func CheckTarget(current, target string) error {
	cur, err := ParseVersion(current)
	if err != nil {
		return err
	}
	tgt, err := ParseVersion(target)
	if err != nil {
		return err
	}
	if tgt.Compare(cur) < 0 {
		return ErrDowngrade
	}
	if tgt.Compare(cur) == 0 {
		return ErrSameVersion
	}
	if tgt.Major != cur.Major {
		return ErrMajorMismatch
	}
	return nil
}

// CheckCompatibility 实现兼容矩阵（design.md §7.2）。
// panelMajor/panelMinor 是 Panel 的 contract major 与 minor；
// agentMajor/agentMinor 来自节点上报的 contract 版本；schemaEqual 表示配置 schema 完全相等。
func CheckCompatibility(panelContractMajor, agentMajor int, panelMinor, agentMinor string, schemaEqual bool) error {
	if panelContractMajor != agentMajor {
		return ErrContractMajor
	}
	pm, err := ParseVersion("1." + panelMinor)
	if err != nil {
		return fmt.Errorf("%w: panel minor %q", ErrCompatibilityBad, panelMinor)
	}
	am, err := ParseVersion("1." + agentMinor)
	if err != nil {
		return fmt.Errorf("%w: agent minor %q", ErrCompatibilityBad, agentMinor)
	}
	if am.Minor > pm.Minor {
		return ErrAgentMinorAhead
	}
	if !schemaEqual {
		return ErrSchemaMismatch
	}
	return nil
}
