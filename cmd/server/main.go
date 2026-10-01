package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/jerry12122/Claude-Code-Mini-App/internal/logging"
	"github.com/jerry12122/Claude-Code-Mini-App/internal/server"
)

func main() {
	syncLogs := logging.Init()
	defer syncLogs()

	srv, err := server.Start(context.Background())
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
	if err := srv.Wait(); err != nil {
		slog.Error(fmt.Sprintf("server 結束: %v", err))
		os.Exit(1)
	}
}
