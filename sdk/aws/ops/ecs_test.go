package ops

import "testing"

func TestECSConfigured(t *testing.T) {
	if ECSConfigured(ECSConfig{}) {
		t.Fatal("empty")
	}
	cfg := ECSConfig{
		Cluster: "c", APIService: "api", WorkerService: "worker",
		APITaskFamily: "api-td", WorkerTaskFamily: "worker-td",
	}
	if !ECSConfigured(cfg) {
		t.Fatal("expected configured")
	}
	cfg.WorkerTaskFamily = ""
	if ECSConfigured(cfg) {
		t.Fatal("incomplete")
	}
}

func TestAWSRollerMode(t *testing.T) {
	r := NewAWSRoller(ECSConfig{Cluster: "c"})
	if r.Mode() != "ecs" {
		t.Fatalf("mode=%s", r.Mode())
	}
}
