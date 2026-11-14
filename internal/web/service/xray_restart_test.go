package service

import (
	"strings"
	"testing"
)

func TestRestartXrayRespectsManualStop(t *testing.T) {
	setupSettingTestDB(t)
	if err := (&SettingService{}).saveSetting("xrayTemplateConfig", "{ not valid json"); err != nil {
		t.Fatalf("seed template: %v", err)
	}
	t.Cleanup(func() { isManuallyStopped.Store(false) })

	isManuallyStopped.Store(true)
	_ = (&XrayService{}).RestartXray(false)

	if !isManuallyStopped.Load() {
		t.Fatal("a non-forced restart cleared a deliberate manual stop and would revive xray")
	}
}

func TestApplyPendingRestartReArmsFlagOnFailure(t *testing.T) {
	setupSettingTestDB(t)
	if err := (&SettingService{}).saveSetting("xrayTemplateConfig", "{ not valid json"); err != nil {
		t.Fatalf("seed template: %v", err)
	}
	t.Cleanup(func() {
		isManuallyStopped.Store(false)
		isNeedXrayRestart.Store(false)
	})
	isManuallyStopped.Store(false)

	svc := &XrayService{}
	svc.SetToNeedRestart()
	svc.ApplyPendingRestart()

	if !isNeedXrayRestart.Load() {
		t.Fatal("a failed restart must re-arm the need-restart flag so the pending config change is retried")
	}
}

func TestDidXrayCrashIgnoresIntentionalRestartWindow(t *testing.T) {
	oldProcess, oldResult := xrayState.snapshot()
	oldManual := isManuallyStopped.Load()
	oldRestarting := isRestarting.Load()
	t.Cleanup(func() {
		xrayState.replace(oldProcess)
		xrayState.storeResult(oldProcess, oldResult)
		isManuallyStopped.Store(oldManual)
		isRestarting.Store(oldRestarting)
	})

	xrayState.replace(nil)
	isManuallyStopped.Store(false)
	isRestarting.Store(true)
	if (&XrayService{}).DidXrayCrash() {
		t.Fatal("an intentional restart window must not be classified as a crash")
	}

	isRestarting.Store(false)
	if !(&XrayService{}).DidXrayCrash() {
		t.Fatal("a stopped, non-manual, non-restarting core must be classified as crashed")
	}
}

func TestLiveStatusRetainsFailedRestartWithoutResamplingCPU(t *testing.T) {
	setupSettingTestDB(t)
	oldProcess, oldResult := xrayState.snapshot()
	oldHeld := xrayState.heldBackReason()
	oldManual := isManuallyStopped.Load()
	t.Cleanup(func() {
		xrayState.replace(oldProcess)
		xrayState.storeResult(oldProcess, oldResult)
		xrayState.holdBack(oldHeld)
		isManuallyStopped.Store(oldManual)
	})
	xrayState.replace(nil)
	isManuallyStopped.Store(true)
	if err := (&SettingService{}).saveSetting("xrayTemplateConfig", "{ malformed"); err != nil {
		t.Fatal(err)
	}
	if err := (&XrayService{}).RestartXray(true); err == nil {
		t.Fatal("invalid template must fail to restart")
	}
	s := &ServerService{lastStatus: &Status{Cpu: 37, RelayLightMode: true}}
	status := s.StatusWithLiveXray(true)
	if status.Cpu != 37 || !status.RelayLightMode || status.Xray.State != Error ||
		!strings.Contains(status.Xray.ErrorMsg, "build Xray configuration") {
		t.Fatalf("unexpected status: CPU=%v Xray=%+v", status.Cpu, status.Xray)
	}
	if !isManuallyStopped.Load() || s.LastStatus().Xray.ErrorMsg != "" {
		t.Fatal("failed restart changed manual stop or mutated the shared status snapshot")
	}
}
