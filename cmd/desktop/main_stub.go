//go:build !windows

package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "桌面版目前只支援 Windows：")
	fmt.Fprintln(os.Stderr, `  go build -ldflags "-H windowsgui" -o claude-miniapp-desktop.exe ./cmd/desktop`)
	os.Exit(1)
}
