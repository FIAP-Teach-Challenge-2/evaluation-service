package main

import "testing"

func TestGetDeterministicBucket(t *testing.T) {
	bucket1 := getDeterministicBucket("user123flagA")
	bucket2 := getDeterministicBucket("user123flagA")
	if bucket1 != bucket2 {
		t.Fatalf("expected getDeterministicBucket to be deterministic, got %d and %d", bucket1, bucket2)
	}
	if bucket1 < 0 || bucket1 > 99 {
		t.Fatalf("expected bucket in range [0, 99], got %d", bucket1)
	}
}

func TestRunEvaluationLogic_FlagDisabled(t *testing.T) {
	app := &App{}
	info := &CombinedFlagInfo{
		Flag: &Flag{Name: "my-flag", IsEnabled: false},
		Rule: nil,
	}
	if app.runEvaluationLogic(info, "user-1") {
		t.Fatal("expected a disabled flag to evaluate to false")
	}
}

func TestRunEvaluationLogic_EnabledNoRule(t *testing.T) {
	app := &App{}
	info := &CombinedFlagInfo{
		Flag: &Flag{Name: "my-flag", IsEnabled: true},
		Rule: nil,
	}
	if !app.runEvaluationLogic(info, "user-1") {
		t.Fatal("expected an enabled flag with no targeting rule to evaluate to true")
	}
}

func TestRunEvaluationLogic_PercentageRuleFullRollout(t *testing.T) {
	app := &App{}
	info := &CombinedFlagInfo{
		Flag: &Flag{Name: "my-flag", IsEnabled: true},
		Rule: &TargetingRule{
			FlagName:  "my-flag",
			IsEnabled: true,
			Rules:     Rule{Type: "PERCENTAGE", Value: float64(100)},
		},
	}
	if !app.runEvaluationLogic(info, "user-1") {
		t.Fatal("expected a 100%% rollout to always evaluate to true")
	}
}
