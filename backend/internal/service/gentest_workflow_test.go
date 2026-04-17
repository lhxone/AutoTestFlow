package service

import "testing"

func TestBuildGenTestWorkflow_Compiles(t *testing.T) {
	_, err := buildGenTestWorkflow(&GenTestService{})
	if err != nil {
		t.Fatalf("buildGenTestWorkflow() error = %v", err)
	}
}
