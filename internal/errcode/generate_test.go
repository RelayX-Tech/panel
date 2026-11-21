package errcode

import (
	"regexp"
	"strings"
	"testing"
)

func sampleCodes() []Code {
	return []Code{
		{Code: "OK", HTTP: 200, I18n: "errors.OK", Owner: "backend-panel", Since: "2026-11-19", Message: "成功"},
		{Code: "AUTH_UNAUTHORIZED", HTTP: 401, I18n: "errors.AUTH_UNAUTHORIZED", Owner: "backend-panel", Since: "2026-11-19", Message: "未认证"},
	}
}

func TestGenerateGoContainsConstantsAndMaps(t *testing.T) {
	out, err := GenerateGo(sampleCodes())
	if err != nil {
		t.Fatalf("generate go: %v", err)
	}
	// go/format 会按最长标识符对齐；归一化空白后只断言 token 序列。
	text := regexp.MustCompile(`\s+`).ReplaceAllString(string(out), " ")
	for _, want := range []string{
		`OK = "OK"`,
		`AUTH_UNAUTHORIZED = "AUTH_UNAUTHORIZED"`,
		`"AUTH_UNAUTHORIZED": 401`,
		`"AUTH_UNAUTHORIZED": "errors.AUTH_UNAUTHORIZED"`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("generated go missing %q\n%s", want, text)
		}
	}
}

func TestGenerateTSAndMessages(t *testing.T) {
	ts, err := GenerateTS(sampleCodes())
	if err != nil {
		t.Fatalf("generate ts: %v", err)
	}
	if !strings.Contains(string(ts), "AUTH_UNAUTHORIZED") || !strings.Contains(string(ts), "401") {
		t.Errorf("generated ts missing code/http: %s", ts)
	}

	msg, err := GenerateMessages(sampleCodes(), "zh-CN")
	if err != nil {
		t.Fatalf("generate messages: %v", err)
	}
	if !strings.Contains(string(msg), `"AUTH_UNAUTHORIZED": "未认证"`) {
		t.Errorf("generated messages missing zh-CN text: %s", msg)
	}
}
