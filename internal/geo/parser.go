// Package geo 解析 Geo 文件并生成分类索引。
//
// 当前实现支持 Xray 纯文本 geosite 格式（每行一个条目，可用 "category: value"
// 指定分类）；二进制 .dat 需要与 core 的 Xray 基线同版本的 protobuf 读取器，
// 在接入前显式返回错误（不猜测、不补假数据）。
package geo

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Category 是分类及条目数。
type Category struct {
	Name         string
	EntriesCount int
}

// Dataset 是解析结果。
type Dataset struct {
	Categories []Category
	Entries    map[string][]string
}

// Supported 返回当前实现支持的文件扩展名。
func Supported(name string) bool {
	return strings.EqualFold(filepath.Ext(name), ".txt")
}

// Parse 解析文件；不支持的格式返回错误。
func Parse(path string) (*Dataset, error) {
	if !Supported(path) {
		return nil, fmt.Errorf("unsupported geo format %q (binary .dat requires Xray baseline reader)", filepath.Ext(path))
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read geo file: %w", err)
	}
	return ParseBytes(data)
}

// ParseBytes 解析文本内容。
func ParseBytes(data []byte) (*Dataset, error) {
	ds := &Dataset{Entries: make(map[string][]string)}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") || strings.HasPrefix(line, "#") {
			continue
		}
		category, value := "default", line
		if idx := strings.Index(line, ":"); idx > 0 {
			category = strings.TrimSpace(line[:idx])
			value = strings.TrimSpace(line[idx+1:])
		}
		if category == "" || value == "" {
			continue
		}
		ds.Entries[category] = append(ds.Entries[category], value)
	}
	if len(ds.Entries) == 0 {
		return nil, fmt.Errorf("geo file contains no entries")
	}
	for name, values := range ds.Entries {
		ds.Categories = append(ds.Categories, Category{Name: name, EntriesCount: len(values)})
	}
	return ds, nil
}

// Entries 返回指定分类的条目；category 为空时返回全部。
func (d *Dataset) EntriesOf(category string) []string {
	if category == "" {
		out := make([]string, 0)
		for _, values := range d.Entries {
			out = append(out, values...)
		}
		return out
	}
	return d.Entries[category]
}
