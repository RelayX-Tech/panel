package response

import "net/http"

// CookieConfig 是会话 Cookie 的可配置属性（design.md §3.2）。
type CookieConfig struct {
	Name          string
	Secure        bool
	MaxAgeSeconds int
}

const defaultSessionMaxAge = 604800 // 7 天，与绝对上限一致

func (c CookieConfig) maxAge() int {
	if c.MaxAgeSeconds <= 0 {
		return defaultSessionMaxAge
	}
	return c.MaxAgeSeconds
}

// SetSessionCookie 下发会话 Cookie：HttpOnly + SameSite=Lax + Path=/（host-only）。
func SetSessionCookie(w http.ResponseWriter, cfg CookieConfig, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     cfg.Name,
		Value:    token,
		Path:     "/",
		MaxAge:   cfg.maxAge(),
		HttpOnly: true,
		Secure:   cfg.Secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// ClearSessionCookie 清除会话 Cookie（Max-Age=0，属性保持一致）。
func ClearSessionCookie(w http.ResponseWriter, cfg CookieConfig) {
	http.SetCookie(w, &http.Cookie{
		Name:     cfg.Name,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   cfg.Secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// ReadSessionCookie 读取指定 Cookie；不存在返回空串。
func ReadSessionCookie(r *http.Request, name string) string {
	c, err := r.Cookie(name)
	if err != nil {
		return ""
	}
	return c.Value
}
