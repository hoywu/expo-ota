package admin

import (
	"context"
	"runtime"
	"strings"
	"testing"

	"go.uber.org/mock/gomock"
)

func TestSystemStatsReturnsRuntimeInfo(t *testing.T) {
	ctrl := gomock.NewController(t)
	svcCtx, _ := newTestSvcCtx(ctrl)

	resp, err := NewSystemStatsLogic(context.Background(), svcCtx).SystemStats()
	if err != nil {
		t.Fatalf("SystemStats returned error: %v", err)
	}
	if resp == nil {
		t.Fatal("SystemStats returned nil resp")
	}

	// Go runtime always allocates some heap and stack after bootstrap.
	if resp.HeapAllocBytes <= 0 {
		t.Errorf("HeapAllocBytes = %d, want > 0", resp.HeapAllocBytes)
	}
	if resp.HeapInUseBytes <= 0 {
		t.Errorf("HeapInUseBytes = %d, want > 0", resp.HeapInUseBytes)
	}
	if resp.HeapSysBytes <= 0 {
		t.Errorf("HeapSysBytes = %d, want > 0", resp.HeapSysBytes)
	}
	if resp.StackInUseBytes <= 0 {
		t.Errorf("StackInUseBytes = %d, want > 0", resp.StackInUseBytes)
	}
	if resp.NumGoroutine <= 0 {
		t.Errorf("NumGoroutine = %d, want > 0", resp.NumGoroutine)
	}
	if !strings.HasPrefix(resp.GoVersion, "go") {
		t.Errorf("GoVersion = %q, want prefix \"go\"", resp.GoVersion)
	}
	if resp.GoVersion != runtime.Version() {
		t.Errorf("GoVersion = %q, want %q", resp.GoVersion, runtime.Version())
	}
	if resp.UptimeSeconds < 0 {
		t.Errorf("UptimeSeconds = %d, want >= 0", resp.UptimeSeconds)
	}
}
