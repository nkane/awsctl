package lambda

import "testing"

func TestParseEnvLines(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    map[string]string
		wantErr bool
	}{
		{
			name: "basic pairs",
			in:   "FOO=bar\nBAZ=qux",
			want: map[string]string{"FOO": "bar", "BAZ": "qux"},
		},
		{
			name: "blank lines skipped and whitespace trimmed",
			in:   "  FOO = bar \n\n\nBAZ=qux\n",
			want: map[string]string{"FOO": "bar", "BAZ": "qux"},
		},
		{
			name: "value may contain '='",
			in:   "URL=https://x?a=1&b=2",
			want: map[string]string{"URL": "https://x?a=1&b=2"},
		},
		{
			name: "empty value allowed",
			in:   "EMPTY=",
			want: map[string]string{"EMPTY": ""},
		},
		{
			name: "empty body yields empty map (clears env)",
			in:   "   \n\n",
			want: map[string]string{},
		},
		{
			name:    "missing equals is an error",
			in:      "JUSTAKEY",
			wantErr: true,
		},
		{
			name:    "empty key is an error",
			in:      "=value",
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseEnvLines(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (result %v)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("len mismatch: got %v want %v", got, tc.want)
			}
			for k, v := range tc.want {
				if got[k] != v {
					t.Fatalf("key %q: got %q want %q", k, got[k], v)
				}
			}
		})
	}
}

func TestParseMemTimeout(t *testing.T) {
	tests := []struct {
		name            string
		mem, timeout    string
		wantMem, wantTo int32
		wantErr         bool
	}{
		{name: "valid", mem: "256", timeout: "30", wantMem: 256, wantTo: 30},
		{name: "min bounds", mem: "128", timeout: "1", wantMem: 128, wantTo: 1},
		{name: "max bounds", mem: "10240", timeout: "900", wantMem: 10240, wantTo: 900},
		{name: "memory below range", mem: "64", timeout: "30", wantErr: true},
		{name: "memory above range", mem: "20000", timeout: "30", wantErr: true},
		{name: "timeout below range", mem: "256", timeout: "0", wantErr: true},
		{name: "timeout above range", mem: "256", timeout: "901", wantErr: true},
		{name: "non-numeric memory", mem: "abc", timeout: "30", wantErr: true},
		{name: "non-numeric timeout", mem: "256", timeout: "x", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mem, to, err := parseMemTimeout(tc.mem, tc.timeout)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got mem=%d to=%d", mem, to)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if mem != tc.wantMem || to != tc.wantTo {
				t.Fatalf("got mem=%d to=%d want mem=%d to=%d", mem, to, tc.wantMem, tc.wantTo)
			}
		})
	}
}
