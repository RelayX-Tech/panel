package settings

import (
	"regexp"
	"strings"

	"github.com/relayx-tech/panel/internal/logic/shared"
	"github.com/relayx-tech/panel/internal/model"
	"github.com/relayx-tech/panel/internal/types"
)

var hostPattern = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)+$`)

func validHost(host string) bool {
	return len(host) <= 253 && hostPattern.MatchString(host)
}

func validTLSMode(mode string) bool {
	switch strings.ToLower(mode) {
	case "panel", "upload", "none":
		return true
	default:
		return false
	}
}

func toSubscriptionHost(h model.SubscriptionHost) types.SubscriptionHost {
	return types.SubscriptionHost{
		Id: h.ID, Host: h.Host, EntryGroupId: h.EntryGroupID, EntryGroupName: h.EntryGroupName,
		TlsStatus: h.TLSStatus, CertNotAfter: shared.TimeStringOpt(h.CertNotAfter),
		CreatedAt: shared.TimeString(h.CreatedAt),
	}
}

func sameHostExists(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "duplicate key")
}
