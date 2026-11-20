package observability

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// UserHash 返回账号的稳定、不可逆摘要，用于日志与错误追踪关联。
// 日志不得落原始账号；同一账号（去首尾空白、大小写不敏感）得到同一摘要。
func UserHash(account string) string {
	normalized := strings.ToLower(strings.TrimSpace(account))
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])[:32]
}
