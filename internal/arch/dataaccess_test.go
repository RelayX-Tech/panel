package arch

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 生成物白名单：goctl 生成文件允许出现 database/sql（实为 sql.Result / sql.Null* 映射）。
var generatedModelFiles = map[string]bool{
	"authusersmodel.go":        true,
	"authusersmodel_gen.go":    true,
	"authsessionsmodel.go":     true,
	"authsessionsmodel_gen.go": true,
	"vars.go":                  true,
}

var forbiddenPersistencePatterns = []string{
	`"database/sql"`,
	"sql.DB",
	"sql.Open(",
	"QueryRowContext",
	"ExecContext",
	"BeginTx",
}

// TestModelAndLogicUseSqlxOnly 静态扫描持久层（AC1 / TC-BE-001、TC-BE-003）。
// 业务代码只允许 go-zero sqlx；database/sql 仅存在于生成物与登记例外（migrate / testsupport / bootstrap / service.go）。
func TestModelAndLogicUseSqlxOnly(t *testing.T) {
	for _, scan := range []struct{ dir, label string }{
		{"../model", "model"},
		{"../logic", "logic"},
	} {
		err := filepath.WalkDir(scan.dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			base := filepath.Base(path)
			if scan.label == "model" && generatedModelFiles[base] {
				return nil
			}

			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			src := string(data)
			for _, pattern := range forbiddenPersistencePatterns {
				if strings.Contains(src, pattern) {
					t.Errorf("%s: contains forbidden persistence pattern %q", path, pattern)
				}
			}
			// model 不自行开启事务（design.md §4.3、TC-BE-006）。
			if scan.label == "model" && strings.Contains(src, "TransactCtx") {
				t.Errorf("%s: model must not open transactions (TransactCtx)", path)
			}
			// 扩展查询禁止字符串拼接 SQL（AC2、TC-BE-007）。
			if base == "auth_queries_ext.go" && strings.Contains(src, "Sprintf") {
				t.Errorf("%s: ext queries must use $n placeholders, not Sprintf", path)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", scan.dir, err)
		}
	}
}

// TestGeneratedModelsPresent 确保两表生成物均在扫描范围内（AC1）。
func TestGeneratedModelsPresent(t *testing.T) {
	for name := range generatedModelFiles {
		if _, err := os.Stat(filepath.Join("..", "model", name)); err != nil {
			t.Errorf("generated model %s missing: %v", name, err)
		}
	}
}
