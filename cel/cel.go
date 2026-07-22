// Package cel: Similarityのパッケージ管理システム
// .celファイルの読み込み・検証を担当
package cel

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// CelFile: .celファイルの内容
type CelFile struct {
	Name         string
	Version      string
	Dependencies []string
}

// Load: .celファイルを読み込む
func Load(dir string) (*CelFile, error) {
	celPath := filepath.Join(dir, "project.cel")
	f, err := os.Open(celPath)
	if err != nil {
		// .celがない場合はnilを返す（任意ファイル）
		return nil, nil
	}
	defer f.Close()

	cel := &CelFile{}
	scanner := bufio.NewScanner(f)
	inDeps := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// 空行・コメントはスキップ
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "name:") {
			cel.Name = strings.TrimSpace(strings.TrimPrefix(line, "name:"))
			inDeps = false
		} else if strings.HasPrefix(line, "version:") {
			cel.Version = strings.TrimSpace(strings.TrimPrefix(line, "version:"))
			inDeps = false
		} else if strings.HasPrefix(line, "dependencies:") {
			inDeps = true
		} else if inDeps && strings.HasPrefix(line, "- ") {
			dep := strings.TrimSpace(strings.TrimPrefix(line, "- "))
			cel.Dependencies = append(cel.Dependencies, dep)
		}
	}

	return cel, scanner.Err()
}

// CheckImports: .iiaファイルで使われているImportが.celのdependenciesに含まれるか検証
func (c *CelFile) CheckImports(imports []string) []string {
	var missing []string
	depSet := make(map[string]bool)
	for _, d := range c.Dependencies {
		depSet[d] = true
	}
	for _, imp := range imports {
		if !depSet[imp] {
			missing = append(missing, imp)
		}
	}
	return missing
}

// Info: .celの内容を表示
func (c *CelFile) Info() string {
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
