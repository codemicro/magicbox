package main

import (
	"fmt"
	"git.tdpain.net/codemicro/magicbox/internal/config"
	"git.tdpain.net/codemicro/magicbox/internal/server"
	"log/slog"
	"os"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	if err := config.Validate(); err != nil {
		return fmt.Errorf("validate config: %w", err)
	}
	return server.ListenAndServe()
}
