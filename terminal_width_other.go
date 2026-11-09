//go:build windows

package main

import (
	"os"
	"strconv"
)

// getTerminalWidth は Windows では COLUMNS 環境変数から取得する。
// 取得できない場合のフォールバックは 80 となる。
func getTerminalWidth() int {
	if cols := os.Getenv("COLUMNS"); cols != "" {
		if n, err := strconv.Atoi(cols); err == nil && n > 0 {
			return n
		}
	}
	return 80
}
