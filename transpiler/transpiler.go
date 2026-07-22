// Package transpiler: .sml（シュガーシンタックス）→ .iia トランスパイラ
package transpiler

import (
	"strings"
)

// 変換テーブル: シュガーキーワード → .iiaキーワード
var sugarToIia = []struct {
	from string
	to   string
}{
	{"Func_pub", "Function_public"},
	{"Func", "Function"},
	{"Var", "Variable"},
	{"App", "Application"},
}

// Transpile: .smlソースを.iiaソースに変換する
func Transpile(src string) string {
	// キーワード置換
	// 単純な文字列置換だと "Func_pub" を "Func" より先に処理しないと壊れる
	// sugarToIiaは長いものから先に定義してあるのでそのまま順番通りに処理
	result := src

	for _, rule := range sugarToIia {
		result = replaceKeyword(result, rule.from, rule.to)
	}

	// {} → [] の変換（外側ブラケットのみ）
	// .smlでは Function{...}{...} のように外側が{} になっている
	// ただし内部ブロック（True{} False{} Body[] 等）は触らない
	// → キーワード直後の { を [ に、対応する } を ] に変換する
	result = convertBrackets(result)

	return result
}

// replaceKeyword: 単語境界を考慮してキーワードを置換
func replaceKeyword(src, from, to string) string {
	var sb strings.Builder
	i := 0
	for i < len(src) {
		if strings.HasPrefix(src[i:], from) {
			// 前が英数字またはアンダースコアなら置換しない
			if i > 0 && isIdentChar(src[i-1]) {
				sb.WriteByte(src[i])
				i++
				continue
			}
			// 後ろが英数字またはアンダースコアなら置換しない
			end := i + len(from)
			if end < len(src) && isIdentChar(src[end]) {
				sb.WriteByte(src[i])
				i++
				continue
			}
			sb.WriteString(to)
			i += len(from)
		} else {
			sb.WriteByte(src[i])
			i++
		}
	}
	return sb.String()
}

func isIdentChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') || c == '_'
}

// convertBrackets: キーワード直後の{...}を[...]に変換する
// シュガー構文では外側ブラケットだけ {} → [] に変える
// 内部ブロック（True{} False{} Body{} 等）は [] に変換しない
//
// 判定ルール: キーワード（大文字始まり識別子）の直後にある { が対象
// ただし receive、True、False、Body、check などの内部構文キーワードは除外
func convertBrackets(src string) string {
	// 外側{}→[]に変換すべきキーワード一覧
	outerKeywords := map[string]bool{
		"Function":         true,
		"Function_public":  true,
		"Variable":         true,
		"Explanation":      true,
		"If":               true,
		"Loop":             true,
		"Async":            true,
		"Await":            true,
		"Mem":              true,
		"Error":            true,
		"Import":           true,
		"Extern":           true,
		"Mutation":         true,
		"GPU":              true,
	}

	runes := []rune(src)
	n := len(runes)
	result := make([]rune, 0, n)

	i := 0
	for i < n {
		// 識別子の開始か確認
		if isUpperLetter(runes[i]) {
			// 識別子を読む
			j := i
			for j < n && isIdentRune(runes[j]) {
				j++
			}
			word := string(runes[i:j])
			result = append(result, runes[i:j]...)
			i = j

			// スペースをスキップ
			k := i
			for k < n && (runes[k] == ' ' || runes[k] == '\t') {
				k++
			}

			// 外側キーワードかつ直後が { なら [] に変換
			if outerKeywords[word] && k < n && runes[k] == '{' {
				// スペースを出力
				result = append(result, runes[i:k]...)
				result = append(result, '[')
				i = k + 1

				// 対応する } を ] に変換（ネスト考慮）
				depth := 1
				for i < n && depth > 0 {
					if runes[i] == '{' {
						depth++
						result = append(result, runes[i])
					} else if runes[i] == '}' {
						depth--
						if depth == 0 {
							result = append(result, ']')
						} else {
							result = append(result, runes[i])
						}
					} else {
						result = append(result, runes[i])
					}
					i++
				}
				continue
			}
		}

		result = append(result, runes[i])
		i++
	}

	return string(result)
}

func isUpperLetter(r rune) bool {
	return r >= 'A' && r <= 'Z'
}

func isIdentRune(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') || r == '_'
}
