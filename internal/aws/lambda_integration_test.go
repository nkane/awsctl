//go:build integration

// LocalStack integration tests for the Lambda write methods (M6 #32–#35).
// See integration_test.go for setup / how to run; these reuse its helpers
// (requireLocalStack, newTestConfig, createPythonNoop, deleteFunction,
// uniqueName, waitFunctionActive).
package aws

import (
	"context"
	"testing"
	"time"
)

func TestLambdaUpdateFunctionEnv(t *testing.T) {
	requireLocalStack(t)
	cfg := newTestConfig(t)
	lc := NewLambdaClient(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	name := uniqueName("awsctl-env")
	createPythonNoop(t, ctx, cfg, name)
	t.Cleanup(func() { deleteFunction(t, cfg, name) })
	if err := waitFunctionActive(ctx, cfg, name); err != nil {
		t.Skipf("function not Active: %v", err)
	}

	want := map[string]string{"FOO": "bar", "LEVEL": "debug"}
	if err := lc.UpdateFunctionEnv(ctx, name, want); err != nil {
		t.Fatalf("UpdateFunctionEnv: %v", err)
	}

	got, err := lc.GetFunction(ctx, name)
	if err != nil {
		t.Fatalf("GetFunction: %v", err)
	}
	if got.Configuration == nil || got.Configuration.Environment == nil {
		t.Fatalf("no environment on function after update")
	}
	vars := got.Configuration.Environment.Variables
	for k, v := range want {
		if vars[k] != v {
			t.Fatalf("env %q: got %q want %q (all: %v)", k, vars[k], v, vars)
		}
	}
}

func TestLambdaUpdateFunctionConfig(t *testing.T) {
	requireLocalStack(t)
	cfg := newTestConfig(t)
	lc := NewLambdaClient(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	name := uniqueName("awsctl-cfg")
	createPythonNoop(t, ctx, cfg, name)
	t.Cleanup(func() { deleteFunction(t, cfg, name) })
	if err := waitFunctionActive(ctx, cfg, name); err != nil {
		t.Skipf("function not Active: %v", err)
	}

	if err := lc.UpdateFunctionConfig(ctx, name, 256, 45); err != nil {
		t.Fatalf("UpdateFunctionConfig: %v", err)
	}

	got, err := lc.GetFunction(ctx, name)
	if err != nil {
		t.Fatalf("GetFunction: %v", err)
	}
	c := got.Configuration
	if c == nil || c.MemorySize == nil || *c.MemorySize != 256 {
		t.Fatalf("memory not updated: %+v", c.MemorySize)
	}
	if c.Timeout == nil || *c.Timeout != 45 {
		t.Fatalf("timeout not updated: %+v", c.Timeout)
	}
}

func TestLambdaPublishVersionAndAlias(t *testing.T) {
	requireLocalStack(t)
	cfg := newTestConfig(t)
	lc := NewLambdaClient(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	name := uniqueName("awsctl-pub")
	createPythonNoop(t, ctx, cfg, name)
	t.Cleanup(func() { deleteFunction(t, cfg, name) })
	if err := waitFunctionActive(ctx, cfg, name); err != nil {
		t.Skipf("function not Active: %v", err)
	}

	ver, err := lc.PublishVersion(ctx, name, "first cut")
	if err != nil {
		t.Fatalf("PublishVersion: %v", err)
	}
	if ver == "" || ver == "$LATEST" {
		t.Fatalf("expected a numeric version, got %q", ver)
	}

	// Create then update the same alias (exercises both branches).
	if err := lc.CreateOrUpdateAlias(ctx, name, "live", ver); err != nil {
		t.Fatalf("CreateOrUpdateAlias (create): %v", err)
	}
	if err := lc.CreateOrUpdateAlias(ctx, name, "live", ver); err != nil {
		t.Fatalf("CreateOrUpdateAlias (update): %v", err)
	}

	d, err := lc.GetFunctionDetail(ctx, name)
	if err != nil {
		t.Fatalf("GetFunctionDetail: %v", err)
	}
	found := false
	for _, a := range d.Aliases {
		if a.Name != nil && *a.Name == "live" {
			found = true
			if a.FunctionVersion == nil || *a.FunctionVersion != ver {
				t.Fatalf("alias points at %v, want %s", a.FunctionVersion, ver)
			}
		}
	}
	if !found {
		t.Fatalf("alias %q not found after CreateOrUpdateAlias", "live")
	}
}

func TestLambdaDeleteFunction(t *testing.T) {
	requireLocalStack(t)
	cfg := newTestConfig(t)
	lc := NewLambdaClient(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	name := uniqueName("awsctl-del")
	createPythonNoop(t, ctx, cfg, name)
	if err := waitFunctionActive(ctx, cfg, name); err != nil {
		t.Skipf("function not Active: %v", err)
	}

	if err := lc.DeleteFunction(ctx, name); err != nil {
		t.Fatalf("DeleteFunction: %v", err)
	}

	// Re-reading the deleted function should fail with not-found.
	if _, err := lc.GetFunction(ctx, name); err == nil {
		t.Fatalf("expected error reading deleted function, got nil")
	} else if !isNotFound(err) {
		t.Logf("delete confirmed via error (non-NotFound type): %v", err)
	}
}
