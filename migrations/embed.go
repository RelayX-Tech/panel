// Package migrations 以 embed 方式打包 SQL 迁移文件，供服务启动时内置迁移器执行。
package migrations

import "embed"

// FS 包含全部迁移 SQL（命名约定 YYYYMMDDHHMM_description.sql）。
//
//go:embed *.sql
var FS embed.FS
