package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"regexp"
	"strings"
	"time"

	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

// TableName 是 goose 版本表；与既有规范一致。
const TableName = "schema_migrations"

var filenameRe = regexp.MustCompile(`^\d{12}_[a-z0-9_]+\.sql$`)

// Up 应用全部未执行迁移；并发实例由 PostgreSQL session advisory lock 串行化。
func Up(ctx context.Context, db *sql.DB, fsys fs.FS) error {
	if err := validateFS(fsys); err != nil {
		return err
	}
	provider, err := newProvider(db, fsys)
	if err != nil {
		return err
	}
	_, err = provider.Up(ctx)
	return err
}

// Down 回滚最近一次迁移；无迁移可回滚时返回 goose.ErrNoNextVersion。
func Down(ctx context.Context, db *sql.DB, fsys fs.FS) error {
	if err := validateFS(fsys); err != nil {
		return err
	}
	provider, err := newProvider(db, fsys)
	if err != nil {
		return err
	}
	_, err = provider.Down(ctx)
	return err
}

// Status 返回迁移状态（按版本升序）。
func Status(ctx context.Context, db *sql.DB, fsys fs.FS) ([]*goose.MigrationStatus, error) {
	provider, err := newProvider(db, fsys)
	if err != nil {
		return nil, err
	}
	return provider.Status(ctx)
}

// Version 返回数据库当前最高迁移版本；未迁移时为 0。
func Version(ctx context.Context, db *sql.DB, fsys fs.FS) (int64, error) {
	provider, err := newProvider(db, fsys)
	if err != nil {
		return 0, err
	}
	return provider.GetDBVersion(ctx)
}

// Check 校验 schema 已应用全部迁移；服务启动只校验、不自动迁移。
func Check(ctx context.Context, db *sql.DB, fsys fs.FS) error {
	statuses, err := Status(ctx, db, fsys)
	if err != nil {
		return fmt.Errorf("read migration status: %w", err)
	}
	for _, status := range statuses {
		if status.State == goose.StateApplied {
			continue
		}
		path := ""
		if status.Source != nil {
			path = status.Source.Path
		}
		return fmt.Errorf("migration %s is %s; run `panel migrate up`", path, status.State)
	}
	return nil
}

// ReadyCheck 返回 /readyz 使用的 schema 版本检查。
func ReadyCheck(db *sql.DB, fsys fs.FS) func() error {
	return func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		return Check(ctx, db, fsys)
	}
}

func newProvider(db *sql.DB, fsys fs.FS) (*goose.Provider, error) {
	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return nil, fmt.Errorf("create migration session locker: %w", err)
	}
	provider, err := goose.NewProvider(
		goose.DialectPostgres,
		db,
		fsys,
		goose.WithTableName(TableName),
		goose.WithSessionLocker(locker),
	)
	if err != nil {
		return nil, fmt.Errorf("create migration provider: %w", err)
	}
	return provider, nil
}

// validateFS 在交给 goose 前校验命名与 Up/Down 结构，保证 CI 可重复。
func validateFS(fsys fs.FS) error {
	names, err := fs.Glob(fsys, "*.sql")
	if err != nil {
		return fmt.Errorf("list migrations: %w", err)
	}
	if len(names) == 0 {
		return fmt.Errorf("no migration files found")
	}
	for _, name := range names {
		if !filenameRe.MatchString(name) {
			return fmt.Errorf("migration %q does not follow YYYYMMDDHHMM_description.sql", name)
		}
		body, err := fs.ReadFile(fsys, name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}
		content := string(body)
		if !strings.Contains(content, "-- +goose Up") {
			return fmt.Errorf("migration %s: missing %q marker", name, "-- +goose Up")
		}
		if !strings.Contains(content, "-- +goose Down") {
			return fmt.Errorf("migration %s: missing %q marker", name, "-- +goose Down")
		}
		if strings.TrimSpace(upSection(content)) == "" {
			return fmt.Errorf("migration %s: empty up section", name)
		}
	}
	return nil
}

func upSection(content string) string {
	const upMarker, downMarker = "-- +goose Up", "-- +goose Down"
	idx := strings.Index(content, upMarker)
	if idx < 0 {
		return ""
	}
	body := content[idx+len(upMarker):]
	if end := strings.Index(body, downMarker); end >= 0 {
		body = body[:end]
	}
	return body
}
