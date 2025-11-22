package auditzip

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"time"
)

type AuditRecorder interface {
	Append(ctx context.Context, entry AuditLog) error
	Last(ctx context.Context, tenantID string) (AuditLog, error)
}

func HashChain(ctx context.Context, rec AuditRecorder, tenantID string, entry AuditLog) (AuditLog, error) {
	prev, _ := rec.Last(ctx, tenantID)
	entry.PrevHash = prev.Hash
	entry.Hash = hashAudit(entry)
	return entry, rec.Append(ctx, entry)
}

func hashAudit(entry AuditLog) string {
	payload := fmt.Sprintf("%s|%s|%s|%s|%s|%s", entry.CorrID, entry.TenantID, entry.Actor, entry.Action, entry.Ts.UTC().Format(time.RFC3339Nano), entry.PrevHash)
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

func CorrelationLogger(logger *slog.Logger, corrID, tenantID string) *slog.Logger {
	if logger == nil {
		logger = slog.Default()
	}
	return logger.With("corrId", corrID, "tenantId", tenantID)
}
