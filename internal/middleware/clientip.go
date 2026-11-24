package middleware

import "context"

type clientIPKey struct{}

// contextWithClientIP 把来源 IP 注入上下文，供审计写入使用。
func contextWithClientIP(ctx context.Context, ip string) context.Context {
	return context.WithValue(ctx, clientIPKey{}, ip)
}

// ClientIPFromContext 读取上下文中的来源 IP；不存在时返回空串。
func ClientIPFromContext(ctx context.Context) string {
	ip, _ := ctx.Value(clientIPKey{}).(string)
	return ip
}
