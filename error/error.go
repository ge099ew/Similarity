// Package error: Similarityのエラーメッセージ
// フォーマット: Error: {行} line, {コード}({説明}). errornumber{番号}
package error

import "fmt"

// エラーナンバー体系
// 1xxxx: 構文エラー
// 2xxxx: 型エラー
// 3xxxx: メモリエラー
// 4xxxx: 実行時エラー
// 5xxxx: Fatalエラー

type SimilarityError struct {
	Line    int
	Code    string
	Message string
	Number  int
}

func (e *SimilarityError) Error() string {
	return fmt.Sprintf(
		"Error: %d line, %s(%s). errornumber%d",
		e.Line, e.Code, e.Message, e.Number,
	)
}

// 構文エラー (1xxxx)
func SyntaxError(line int, code, msg string, num int) *SimilarityError {
	return &SimilarityError{line, code, msg, 10000 + num}
}

// 型エラー (2xxxx)
func TypeError(line int, code, msg string, num int) *SimilarityError {
	return &SimilarityError{line, code, msg, 20000 + num}
}

// メモリエラー (3xxxx)
func MemoryError(line int, code, msg string, num int) *SimilarityError {
	return &SimilarityError{line, code, msg, 30000 + num}
}

// 実行時エラー (4xxxx)
func RuntimeError(line int, code, msg string, num int) *SimilarityError {
	return &SimilarityError{line, code, msg, 40000 + num}
}

// Fatalエラー (5xxxx)
func FatalError(line int, code, msg string, num int) *SimilarityError {
	return &SimilarityError{line, code, msg, 50000 + num}
}
