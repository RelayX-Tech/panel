package errcode

import (
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Code 是错误码注册表条目；api/error-codes.yaml 是唯一来源。
type Code struct {
	Code       string `yaml:"code"`
	HTTP       int    `yaml:"http"`
	I18n       string `yaml:"i18n"`
	Owner      string `yaml:"owner"`
	Since      string `yaml:"since"`
	Deprecated bool   `yaml:"deprecated"`
	Message    string `yaml:"message"`
	MessageEn  string `yaml:"messageEn,omitempty"`
	ReplacedBy string `yaml:"replacedBy,omitempty"`
}

var codeRe = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

// Load 解析并校验注册表 YAML。
func Load(data []byte) ([]Code, error) {
	var codes []Code
	if err := yaml.Unmarshal(data, &codes); err != nil {
		return nil, fmt.Errorf("parse error-codes.yaml: %w", err)
	}
	if err := ValidateAll(codes); err != nil {
		return nil, err
	}
	return codes, nil
}

// LoadFile 从文件读取注册表。
func LoadFile(path string) ([]Code, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return Load(data)
}

// Validate 校验单个错误码条目。
func (c Code) Validate() error {
	if c.Code == "" {
		return fmt.Errorf("code is required")
	}
	if !codeRe.MatchString(c.Code) {
		return fmt.Errorf("code %q must be upper snake case", c.Code)
	}
	if !allowedHTTP(c.HTTP) {
		return fmt.Errorf("code %q has unsupported http status %d", c.Code, c.HTTP)
	}
	if c.I18n != "errors."+c.Code {
		return fmt.Errorf("code %q i18n = %q, want errors.%s", c.Code, c.I18n, c.Code)
	}
	if strings.TrimSpace(c.Owner) == "" {
		return fmt.Errorf("code %q owner is required", c.Code)
	}
	if strings.TrimSpace(c.Since) == "" {
		return fmt.Errorf("code %q since is required", c.Code)
	}
	if strings.TrimSpace(c.Message) == "" {
		return fmt.Errorf("code %q message is required", c.Code)
	}
	if c.Deprecated && strings.TrimSpace(c.ReplacedBy) == "" {
		return fmt.Errorf("code %q is deprecated but replacedBy is empty", c.Code)
	}
	return nil
}

// ValidateAll 校验注册表整体：每条合法且 code 唯一。
func ValidateAll(codes []Code) error {
	if len(codes) == 0 {
		return fmt.Errorf("error code registry is empty")
	}
	seen := make(map[string]bool, len(codes))
	for _, c := range codes {
		if err := c.Validate(); err != nil {
			return err
		}
		if seen[c.Code] {
			return fmt.Errorf("duplicate error code %q", c.Code)
		}
		seen[c.Code] = true
	}
	return nil
}

func allowedHTTP(code int) bool {
	switch code {
	case http.StatusOK, http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden,
		http.StatusNotFound, http.StatusConflict, http.StatusRequestEntityTooLarge,
		http.StatusUnprocessableEntity, http.StatusUpgradeRequired, http.StatusTooManyRequests,
		http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}
