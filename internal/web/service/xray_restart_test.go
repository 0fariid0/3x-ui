package service

import (
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
	oldProcess := p
	oldManual := isManuallyStopped.Load()
	oldRestarting := isRestarting.Load()
	t.Cleanup(func() {
		p = oldProcess
		isManuallyStopped.Store(oldManual)
		isRestarting.Store(oldRestarting)
	})

	p = nil
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
