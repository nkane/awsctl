// Command awsctl is a TUI for managing AWS Lambda functions and querying
// DynamoDB tables. Run with --unsafe to enable mutating operations.
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime/debug"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/nkane/awsctl/internal/ui"
)

// Version is set at build time via -ldflags.
var Version = "dev"

func main() {
	var (
		profile = flag.String("profile", os.Getenv("AWS_PROFILE"), "AWS profile name (defaults to AWS_PROFILE / default)")
		region  = flag.String("region", os.Getenv("AWS_REGION"), "AWS region (defaults to AWS_REGION / profile config)")
		unsafe  = flag.Bool("unsafe", false, "enable write/destructive operations (gated behind confirm modals)")
		logPath = flag.String("log", defaultLogPath(), "path to log file (TUI cannot use stdout)")
		version = flag.Bool("version", false, "print version and exit")
	)
	flag.Parse()

	if *version {
		fmt.Println("awsctl", Version)
		return
	}

	logger, closer, err := newLogger(*logPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "awsctl: setup logger:", err)
		os.Exit(1)
	}
	defer closer()

	logger.Info("awsctl starting", "version", Version, "unsafe", *unsafe, "profile", *profile, "region", *region)

	app := ui.NewApp(ui.Options{
		Profile: *profile,
		Region:  *region,
		Unsafe:  *unsafe,
		Logger:  logger,
	})

	p := tea.NewProgram(app, tea.WithAltScreen(), tea.WithMouseCellMotion())
	// Recover panics so the stack trace lands in the log file instead of being
	// eaten by the alt screen.
	defer func() {
		if r := recover(); r != nil {
			logger.Error("panic", "value", fmt.Sprint(r), "stack", string(debug.Stack()))
			fmt.Fprintf(os.Stderr, "awsctl: panic: %v\n%s\n", r, debug.Stack())
			os.Exit(2)
		}
	}()
	if _, err := p.Run(); err != nil {
		logger.Error("tea program exited with error", "err", err)
		fmt.Fprintln(os.Stderr, "awsctl:", err)
		os.Exit(1)
	}
}

// newLogger writes structured logs to a file. The TUI takes over stdout/stderr,
// so a file sink is the only sane default.
func newLogger(path string) (*slog.Logger, func(), error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, nil, err
	}
	h := slog.NewJSONHandler(f, &slog.HandlerOptions{Level: slog.LevelInfo})
	return slog.New(h), func() { _ = f.Close() }, nil
}

func defaultLogPath() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "awsctl", "awsctl.log")
}
