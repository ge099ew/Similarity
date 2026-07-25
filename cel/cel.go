// Package cel: Similarityのパッケージ管理システム
// .celファイルの読み込み・検証を担当
package cel

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// CelFile: .celファイルの内容
type CelFile struct {
	Name         string
	Version      string
	Dependencies []string
}

// バージョン形式チェック: x.y.z
var versionRegex = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

// Load: .celファイルを読み込む
func Load(dir string) (*CelFile, error) {
	celPath := filepath.Join(dir, "project.cel")
	f, err := os.Open(celPath)
	if err != nil {
		// .celがない場合は空のCelFileを返す
		return &CelFile{}, nil
	}
	defer f.Close()

	cel := &CelFile{}
	scanner := bufio.NewScanner(f)
	inDeps := false
	lineNum := 0
	depSet := make(map[string]struct{})
	var errs []string

	knownKeys := map[string]bool{
		"name": true, "version": true, "dependencies": true,
	}

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// 空行・コメントはスキップ
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// dependenciesブロック内のリスト
		if inDeps && strings.HasPrefix(line, "- ") {
			dep := strings.TrimSpace(strings.TrimPrefix(line, "- "))
			if _, dup := depSet[dep]; dup {
				errs = append(errs, fmt.Sprintf("project.cel:%d: duplicate dependency: %s", lineNum, dep))
			} else {
				depSet[dep] = struct{}{}
				cel.Dependencies = append(cel.Dependencies, dep)
			}
			continue
		}
		inDeps = false

		// key: value形式チェック
		if !strings.Contains(line, ":") {
			errs = append(errs, fmt.Sprintf("project.cel:%d: expected \"key: value\", got: %s", lineNum, line))
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		// 未知のキー検出
		if !knownKeys[key] {
			errs = append(errs, fmt.Sprintf("project.cel:%d: unknown key: %s", lineNum, key))
			continue
		}

		switch key {
		case "name":
			cel.Name = val
		case "version":
			if val != "" && !versionRegex.MatchString(val) {
				errs = append(errs, fmt.Sprintf("project.cel:%d: invalid version format: %s (expected x.y.z)", lineNum, val))
			}
			cel.Version = val
		case "dependencies":
			inDeps = true
		}
	}

	if err := scanner.Err(); err != nil {
		return cel, err
	}

	if len(errs) > 0 {
		return cel, fmt.Errorf("%s", strings.Join(errs, "\n"))
	}

	return cel, nil
}

// CheckImports: .iiaファイルで使われているImportが.celのdependenciesに含まれるか検証
func (c *CelFile) CheckImports(imports []string) []string {
	var missing []string
	depSet := make(map[string]struct{})
	for _, d := range c.Dependencies {
		depSet[d] = struct{}{}
	}
	for _, imp := range imports {
		if _, ok := depSet[imp]; !ok {
			missing = append(missing, imp)
		}
	}
	return missing
}

// Info: .celの内容を表示
func (c *CelFile) Info() string {
	if c.Name == "" && c.Version == "" && len(c.Dependencies) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Project : %s\n", c.Name))
	sb.WriteString(fmt.Sprintf("Version : %s\n", c.Version))
	if len(c.Dependencies) > 0 {
		sb.WriteString("Dependencies:\n")
		for _, d := range c.Dependencies {
			sb.WriteString(fmt.Sprintf("  - %s\n", d))
		}
	} else {
		sb.WriteString("Dependencies: none\n")
	}
	return sb.String()
}
