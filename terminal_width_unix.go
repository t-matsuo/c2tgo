//go:build !windows

package main

import (
	"os"
	"strconv"
	"syscall"
	"unsafe"
)

// winsize は TIOCGWINSZ ioctl で取得する端末サイズを表す。
type winsize struct {
	Row    uint16
	Col    uint16
	Xpixel uint16
	Ypixel uint16
}

// getTerminalWidth は標準出力のターミナル幅を取得する。
// ターミナルが検出できない場合は COLUMNS 環境変数、それもない場合は 80 を返す。
func getTerminalWidth() int {
	ws := &winsize{}
	retCode, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		os.Stdout.Fd(),
		uintptr(syscall.TIOCGWINSZ),
		uintptr(unsafe.Pointer(ws)),
	)
	if retCode != 0 || errno != 0 {
		return terminalWidthFallback()
	}
	if ws.Col == 0 {
		return terminalWidthFallback()
	}
	return int(ws.Col)
}

// terminalWidthFallback は環境変数 COLUMNS から端末幅を取得する。
func terminalWidthFallback() int {
	if cols := os.Getenv("COLUMNS"); cols != "" {
		if n, err := strconv.Atoi(cols); err == nil && n > 0 {
			return n
		}
	}
	return 80
}
