// Command coveragegate 基于 go test 覆盖率剖面（coverprofile）按包校验阈值。
//
// 门禁语义（棘轮基线）：
//   - 每个非排除包的覆盖率不得低于 coverage-thresholds.yml 中登记的阈值；
//   - 阈值 = 引入门禁时的当前覆盖率（向下取整到 0.1%），只升不降；
//   - goctl 生成的薄适配层（internal/handler/**）与测试辅助（internal/testsupport）排除；
//   - 低于阈值时输出未达标包清单并以退出码 1 失败。
//
// 用法：
//
//	go run ./cmd/coveragegate -profile coverage.out -thresholds coverage-thresholds.yml
//	go run ./cmd/coveragegate -profile coverage.out -thresholds coverage-thresholds.yml -update
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// PackageCoverage 是单个包在覆盖率剖面中的聚合结果。
type PackageCoverage struct {
	Package string
	Covered int
	Total   int
}

// Percent 返回语句覆盖率（0-100）；无语句的包视为 100%。
func (p PackageCoverage) Percent() float64 {
	if p.Total == 0 {
		return 100
	}
	return float64(p.Covered) / float64(p.Total) * 100
}

// Thresholds 是覆盖率门禁配置：默认阈值 + 按包覆盖 + 排除规则。
type Thresholds struct {
	Default  float64            `yaml:"default" json:"default"`
	Exclude  []string           `yaml:"exclude,omitempty" json:"exclude,omitempty"`
	Packages map[string]float64 `yaml:"packages,omitempty" json:"packages,omitempty"`
}

// Failure 是一个未达标的包。
type Failure struct {
	Package   string
	Percent   float64
	Threshold float64
}

// ParseProfile 解析 go test coverprofile，按包聚合语句数与已覆盖语句数。
// module 用于把完整导入路径裁剪为仓库内相对路径（如 internal/logic/nodes）。
func ParseProfile(r io.Reader, module string) (map[string]PackageCoverage, error) {
	prefix := strings.TrimSuffix(module, "/") + "/"
	pkgs := make(map[string]PackageCoverage)

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "mode:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 3 {
			return nil, fmt.Errorf("malformed coverage line %q", line)
		}
		stmts, err := strconv.Atoi(fields[1])
		if err != nil {
			return nil, fmt.Errorf("malformed statement count in %q: %w", line, err)
		}
		count, err := strconv.Atoi(fields[2])
		if err != nil {
			return nil, fmt.Errorf("malformed hit count in %q: %w", line, err)
		}

		sep := strings.LastIndex(fields[0], ":")
		if sep <= 0 {
			return nil, fmt.Errorf("malformed coverage location %q", fields[0])
		}
		file := strings.ReplaceAll(fields[0][:sep], "\\", "/")
		pkg := strings.TrimPrefix(path.Dir(file), prefix)

		agg := pkgs[pkg]
		agg.Package = pkg
		agg.Total += stmts
		if count > 0 {
			agg.Covered += stmts
		}
		pkgs[pkg] = agg
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return pkgs, nil
}

func normalizedExclude(e string) string {
	return strings.TrimSuffix(strings.ReplaceAll(e, "\\", "/"), "/")
}

// isExcluded 匹配包自身或其后代（internal/handler 命中 internal/handler/auth 等）。
func isExcluded(pkg string, exclusions []string) bool {
	for _, e := range exclusions {
		e = normalizedExclude(e)
		if e == "" {
			continue
		}
		if pkg == e || strings.HasPrefix(pkg, e+"/") {
			return true
		}
	}
	return false
}

// Evaluate 返回低于各自阈值的包（按包名排序）；无语句与排除包不参与。
func Evaluate(pkgs map[string]PackageCoverage, th Thresholds) []Failure {
	var failures []Failure
	for name, pc := range pkgs {
		if pc.Total == 0 || isExcluded(name, th.Exclude) {
			continue
		}
		limit, ok := th.Packages[name]
		if !ok {
			limit = th.Default
		}
		if pc.Percent()+1e-9 < limit {
			failures = append(failures, Failure{Package: name, Percent: pc.Percent(), Threshold: limit})
		}
	}
	sort.Slice(failures, func(i, j int) bool { return failures[i].Package < failures[j].Package })
	return failures
}

// GenerateThresholds 以当前覆盖率为新基线（向下取整到 0.1%），保留默认值与排除规则。
func GenerateThresholds(pkgs map[string]PackageCoverage, base Thresholds) Thresholds {
	out := Thresholds{
		Default:  base.Default,
		Exclude:  append([]string(nil), base.Exclude...),
		Packages: make(map[string]float64, len(pkgs)),
	}
	for name, pc := range pkgs {
		if pc.Total == 0 || isExcluded(name, base.Exclude) {
			continue
		}
		out.Packages[name] = math.Floor(pc.Percent()*10) / 10
	}
	return out
}

// LoadThresholds 读取 YAML 阈值配置。
func LoadThresholds(file string) (Thresholds, error) {
	var th Thresholds
	data, err := os.ReadFile(file)
	if err != nil {
		return th, err
	}
	if err := yaml.Unmarshal(data, &th); err != nil {
		return th, err
	}
	if th.Packages == nil {
		th.Packages = map[string]float64{}
	}
	return th, nil
}

const thresholdsHeader = `# 覆盖率门禁基线（棘轮）
# 每包阈值 = 引入门禁时的当前覆盖率（向下取整到 0.1%），只升不降。
# 覆盖率改进后运行 ` + "`task panel:coverage:update`" + ` 重新生成。
# 排除 goctl 生成的薄适配层（internal/handler/**）与测试辅助（internal/testsupport）。
`

// SaveThresholds 写入 YAML 阈值配置（附说明头）。
func SaveThresholds(file string, th Thresholds) error {
	if th.Packages == nil {
		th.Packages = map[string]float64{}
	}
	data, err := yaml.Marshal(th)
	if err != nil {
		return err
	}
	return os.WriteFile(file, append([]byte(thresholdsHeader), data...), 0o644)
}

// run 是命令入口（可测试）。
func run(args []string) int {
	fs := flag.NewFlagSet("coveragegate", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	profilePath := fs.String("profile", "coverage.out", "coverage profile path")
	thresholdsPath := fs.String("thresholds", "coverage-thresholds.yml", "thresholds config path")
	module := fs.String("module", "github.com/relayx-tech/panel", "module import path to strip")
	update := fs.Bool("update", false, "regenerate thresholds from profile")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	file, err := os.Open(*profilePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "coveragegate: %v\n", err)
		return 2
	}
	defer file.Close()

	pkgs, err := ParseProfile(file, *module)
	if err != nil {
		fmt.Fprintf(os.Stderr, "coveragegate: %v\n", err)
		return 2
	}

	th, err := LoadThresholds(*thresholdsPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "coveragegate: %v\n", err)
		return 2
	}

	if *update {
		if err := SaveThresholds(*thresholdsPath, GenerateThresholds(pkgs, th)); err != nil {
			fmt.Fprintf(os.Stderr, "coveragegate: %v\n", err)
			return 2
		}
		fmt.Printf("coveragegate: 已更新 %s\n", *thresholdsPath)
		return 0
	}

	printReport(os.Stdout, pkgs, th)
	failures := Evaluate(pkgs, th)
	if len(failures) > 0 {
		fmt.Fprintf(os.Stderr, "\n❌ 覆盖率门禁失败：%d 个包低于阈值\n", len(failures))
		for _, f := range failures {
			fmt.Fprintf(os.Stderr, "  - %s: %.1f%% < %.1f%%\n", f.Package, f.Percent, f.Threshold)
		}
		return 1
	}
	fmt.Println("\n✅ 覆盖率门禁通过")
	return 0
}

func printReport(w io.Writer, pkgs map[string]PackageCoverage, th Thresholds) {
	names := make([]string, 0, len(pkgs))
	for name := range pkgs {
		names = append(names, name)
	}
	sort.Strings(names)

	fmt.Fprintf(w, "%-48s %8s %8s  %s\n", "PACKAGE", "COVER", "LIMIT", "STATUS")
	for _, name := range names {
		pc := pkgs[name]
		switch {
		case isExcluded(name, th.Exclude):
			fmt.Fprintf(w, "%-48s %7.1f%% %8s  %s\n", name, pc.Percent(), "-", "skip")
		case pc.Total == 0:
			fmt.Fprintf(w, "%-48s %8s %8s  %s\n", name, "-", "-", "skip")
		default:
			limit, ok := th.Packages[name]
			if !ok {
				limit = th.Default
			}
			status := "ok"
			if pc.Percent()+1e-9 < limit {
				status = "FAIL"
			}
			fmt.Fprintf(w, "%-48s %7.1f%% %7.1f%%  %s\n", name, pc.Percent(), limit, status)
		}
	}
}

func main() {
	os.Exit(run(os.Args[1:]))
}
