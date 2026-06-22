package admin

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/hoywu/expo-ota/server/api/admin/internal/reqmeta"
	"github.com/hoywu/expo-ota/server/api/admin/internal/svc"
	"github.com/hoywu/expo-ota/server/db/models"
	"github.com/zeromicro/go-zero/core/logc"
)

// writeAudit records an admin write operation (§11.5). It is best-effort:
// failures are logged and never fail the originating request.
func writeAudit(ctx context.Context, svcCtx *svc.ServiceContext, action, appId, targetType, targetId string, payload map[string]any) {
	row := &models.AuditLogs{
		Action:     action,
		AppId:      nullString(appId),
		TargetType: nullString(targetType),
		TargetId:   nullString(targetId),
		Payload:    "{}",
		OccurredAt: time.Now().UTC(),
	}

	if actorId, err := userIDFromContext(ctx); err == nil {
		row.ActorUserId = nullString(actorId)
	}
	if meta, ok := reqmeta.FromContext(ctx); ok {
		row.Ip = nullString(meta.IP)
		row.UserAgent = nullString(meta.UserAgent)
		row.RequestId = nullString(meta.RequestID)
	}
	if payload != nil {
		if data, err := json.Marshal(payload); err == nil {
			row.Payload = string(data)
		}
	}

	if _, err := svcCtx.AuditLogsModel.Insert(ctx, row); err != nil {
		logc.Errorf(ctx, "write audit log failed: action=%s err=%v", action, err)
	}
}

func nullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}
