package aws

import (
	"os"
	"path/filepath"
	"testing"
)

// TestListProfiles points HOME at a temp dir containing a fake ~/.aws/config
// and ~/.aws/credentials, then asserts the union of profile names is parsed
// correctly. Runs without docker/LocalStack.
func TestListProfiles(t *testing.T) {
	tmp := t.TempDir()
	awsDir := filepath.Join(tmp, ".aws")
	if err := os.MkdirAll(awsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	cfg := `[default]
region = us-east-1

[profile dev]
region = us-west-2

[profile prod]
region = eu-west-1
`
	creds := `[default]
aws_access_key_id = AKIA000
aws_secret_access_key = secret

[ci]
aws_access_key_id = AKIA111
aws_secret_access_key = secret
`
	if err := os.WriteFile(filepath.Join(awsDir, "config"), []byte(cfg), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(awsDir, "credentials"), []byte(creds), 0o644); err != nil {
		t.Fatalf("write credentials: %v", err)
	}

	t.Setenv("HOME", tmp)

	got, err := ListProfiles()
	if err != nil {
		t.Fatalf("ListProfiles: %v", err)
	}

	want := map[string]bool{"default": true, "dev": true, "prod": true, "ci": true}
	if len(got) != len(want) {
		t.Fatalf("got %d profiles, want %d (%v)", len(got), len(want), got)
	}
	for _, p := range got {
		if !want[p] {
			t.Errorf("unexpected profile %q in result", p)
		}
		delete(want, p)
	}
	for p := range want {
		t.Errorf("missing profile %q", p)
	}
}
