package requestid

import (
	"context"
	"crypto/rand"
	"math/big"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const (
	// HeaderName 是请求/响应中的追踪 ID 头，契约见 design.md §3.2。
	HeaderName = "X-Request-Id"

	crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
)

var uuidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

type ctxKey struct{}

// NewContext 把 requestId 注入请求上下文。
func NewContext(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// FromContext 读取请求上下文中的 requestId；不存在时返回空串。
func FromContext(ctx context.Context) string {
	id, _ := ctx.Value(ctxKey{}).(string)
	return id
}

// Middleware 读取合法的 ULID 形式 X-Request-Id，缺失或非法时生成新的 ULID，
// 并写入响应头与请求上下文。
func Middleware() func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			id := Normalize(r.Header.Get(HeaderName))
			if id == "" {
				id = Generate()
			}
			w.Header().Set(HeaderName, id)
			next(w, r.WithContext(NewContext(r.Context(), id)))
		}
	}
}

// Normalize 校验 ULID（26 位 Crockford Base32）或 UUID（36 位带连字符）；
// 合法时返回规范形式（ULID 大写、UUID 小写），否则返回空串。
func Normalize(raw string) string {
	raw = strings.TrimSpace(raw)
	if uuidRe.MatchString(raw) {
		return strings.ToLower(raw)
	}
	if len(raw) != 26 {
		return ""
	}
	upper := strings.ToUpper(raw)
	for _, c := range upper {
		if !strings.ContainsRune(crockford, c) {
			return ""
		}
	}
	// 128-bit ULID: 首字符只能取 0-7
	if upper[0] > '7' {
		return ""
	}
	return upper
}

// Generate 生成一个 ULID：48 位毫秒时间戳 + 80 位密码学随机数。
func Generate() string {
	var id [16]byte
	ms := uint64(time.Now().UnixMilli())
	for i := 5; i >= 0; i-- {
		id[i] = byte(ms)
		ms >>= 8
	}
	_, _ = rand.Read(id[6:])
	return encode(id)
}

func encode(id [16]byte) string {
	// 128 位 → 26 个 Crockford Base32 字符（高位补零）
	n := new(big.Int).SetBytes(id[:])
	mask := big.NewInt(31)
	digit := new(big.Int)
	out := make([]byte, 26)
	for i := 25; i >= 0; i-- {
		digit.And(n, mask)
		out[i] = crockford[digit.Int64()]
		n.Rsh(n, 5)
	}
	return string(out)
}
