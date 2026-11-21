package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/crypto/argon2"
)

// Argon2id 参数（design.md §4.4 / OWASP 推荐档位）。
const (
	argonMemory  = 19456 // KiB
	argonTime    = 2
	argonThreads = 1
	argonSaltLen = 16
	argonKeyLen  = 32
)

var errInvalidHash = errors.New("invalid password hash")

// HashPassword 生成 Argon2id PHC 字符串（每用户独立 16 字节盐）。
func HashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// VerifyPassword 校验明文密码与 PHC 哈希；不匹配返回 (false, nil)。
func VerifyPassword(password, encoded string) (bool, error) {
	params, salt, expected, err := parsePHC(encoded)
	if err != nil {
		return false, err
	}
	actual := argon2.IDKey([]byte(password), salt, params.time, params.memory, params.threads, uint32(len(expected)))
	return subtle.ConstantTimeCompare(actual, expected) == 1, nil
}

type argonParams struct {
	memory  uint32
	time    uint32
	threads uint8
}

func parsePHC(encoded string) (argonParams, []byte, []byte, error) {
	var p argonParams
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return p, nil, nil, errInvalidHash
	}
	if parts[2] != fmt.Sprintf("v=%d", argon2.Version) {
		return p, nil, nil, errInvalidHash
	}

	for _, kv := range strings.Split(parts[3], ",") {
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			return p, nil, nil, errInvalidHash
		}
		n, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return p, nil, nil, errInvalidHash
		}
		switch k {
		case "m":
			p.memory = uint32(n)
		case "t":
			p.time = uint32(n)
		case "p":
			if n > 255 {
				return p, nil, nil, errInvalidHash
			}
			p.threads = uint8(n)
		default:
			return p, nil, nil, errInvalidHash
		}
	}
	if p.memory == 0 || p.time == 0 || p.threads == 0 {
		return p, nil, nil, errInvalidHash
	}
	if p.memory > 1<<20 || p.time > 64 { // 防止恶意哈希导致资源耗尽
		return p, nil, nil, errInvalidHash
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) < 8 {
		return p, nil, nil, errInvalidHash
	}
	key, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(key) == 0 {
		return p, nil, nil, errInvalidHash
	}
	return p, salt, key, nil
}

var (
	dummyHashOnce sync.Once
	dummyHash     string
)

// VerifyDummy 对固定假哈希执行一次等价成本校验，用于账号不存在时防时序枚举；
// 返回值恒为 (false, nil)。
func VerifyDummy(password string) (bool, error) {
	dummyHashOnce.Do(func() {
		dummyHash, _ = HashPassword("relayx-dummy-password")
	})
	_, _ = VerifyPassword(password, dummyHash)
	return false, nil
}
