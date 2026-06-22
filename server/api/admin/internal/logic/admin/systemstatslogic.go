// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"context"
	"runtime"
	"time"

	"github.com/hoywu/expo-ota/server/api/admin/internal/svc"
	"github.com/hoywu/expo-ota/server/api/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// startTime is captured when the admin logic package is first loaded.
// The admin package is initialized during main startup via svc.NewServiceContext
// and adminlogic.BootstrapInitialAdmin, so this is a close approximation of
// process start time suitable for uptime display.
var startTime = time.Now()

type SystemStatsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSystemStatsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SystemStatsLogic {
	return &SystemStatsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SystemStatsLogic) SystemStats() (resp *types.SystemStatsResp, err error) {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)

	return &types.SystemStatsResp{
		HeapAllocBytes:  int64(ms.HeapAlloc),
		HeapInUseBytes:  int64(ms.HeapInuse),
		HeapSysBytes:    int64(ms.HeapSys),
		StackInUseBytes: int64(ms.StackInuse),
		NumGC:           int64(ms.NumGC),
		NumGoroutine:    int64(runtime.NumGoroutine()),
		GoVersion:       runtime.Version(),
		UptimeSeconds:   int64(time.Since(startTime).Seconds()),
	}, nil
}
