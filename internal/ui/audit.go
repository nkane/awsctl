package ui

import (
	"context"
	"log/slog"
)

// auditLog records a confirmed mutation to logger at INFO level under a single
// "audit" attr group, so write actions are traceable in the log file. A nil
// logger is a no-op (tests pass a discard logger). result is "confirmed",
// "cancelled", "ok", or "error"; extra key/value attrs may be appended and land
// inside the "audit" group alongside action/target/result.
func auditLog(logger *slog.Logger, action, target, result string, attrs ...any) {
	if logger == nil {
		return
	}
	group := []any{
		slog.String("action", action),
		slog.String("target", target),
		slog.String("result", result),
	}
	group = append(group, attrs...)
	logger.LogAttrs(context.Background(), slog.LevelInfo, "audit", slog.Group("audit", group...))
}
