package target_test

import (
	"testing"

	"github.com/ValeryCherneykin/ned/internal/target"
)

func TestParse_SSH(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		input      string
		wantUser   string
		wantHost   string
		wantPort   string
		wantPath   string
		wantScheme string
	}{
		{
			name:       "full with user and port",
			input:      "root@192.168.1.10:2222:/etc/.env",
			wantUser:   "root",
			wantHost:   "192.168.1.10",
			wantPort:   "2222",
			wantPath:   "/etc/.env",
			wantScheme: target.SchemeSSH,
		},
		{
			name:       "user and host only",
			input:      "deploy@prod.example.com:/app/.env",
			wantUser:   "deploy",
			wantHost:   "prod.example.com",
			wantPort:   target.DefaultPort,
			wantPath:   "/app/.env",
			wantScheme: target.SchemeSSH,
		},
		{
			name:       "host only no user",
			input:      "10.0.0.1:/tmp/test.txt",
			wantHost:   "10.0.0.1",
			wantPort:   target.DefaultPort,
			wantPath:   "/tmp/test.txt",
			wantScheme: target.SchemeSSH,
		},
		{
			name:       "deeply nested path",
			input:      "root@host:/a/b/c/d/file.conf",
			wantUser:   "root",
			wantHost:   "host",
			wantPort:   target.DefaultPort,
			wantPath:   "/a/b/c/d/file.conf",
			wantScheme: target.SchemeSSH,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := target.Parse(tc.input)
			if err != nil {
				t.Fatalf("Parse(%q) unexpected error: %v", tc.input, err)
			}

			if got.Scheme != tc.wantScheme {
				t.Errorf("Scheme: got %q, want %q", got.Scheme, tc.wantScheme)
			}

			if tc.wantUser != "" && got.User != tc.wantUser {
				t.Errorf("User: got %q, want %q", got.User, tc.wantUser)
			}

			if got.Host != tc.wantHost {
				t.Errorf("Host: got %q, want %q", got.Host, tc.wantHost)
			}

			if got.Port != tc.wantPort {
				t.Errorf("Port: got %q, want %q", got.Port, tc.wantPort)
			}

			if got.RemotePath != tc.wantPath {
				t.Errorf("RemotePath: got %q, want %q", got.RemotePath, tc.wantPath)
			}
		})
	}
}

func TestParse_Docker(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		input         string
		wantContainer string
		wantPath      string
	}{
		{
			name:          "basic docker",
			input:         "docker://my-container:/app/config.json",
			wantContainer: "my-container",
			wantPath:      "/app/config.json",
		},
		{
			name:          "docker with dashes",
			input:         "docker://ned-test:/etc/nginx/nginx.conf",
			wantContainer: "ned-test",
			wantPath:      "/etc/nginx/nginx.conf",
		},
		{
			name:          "docker deeply nested",
			input:         "docker://app:/a/b/c/file.txt",
			wantContainer: "app",
			wantPath:      "/a/b/c/file.txt",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := target.Parse(tc.input)
			if err != nil {
				t.Fatalf("Parse(%q) unexpected error: %v", tc.input, err)
			}

			if !got.IsDocker() {
				t.Errorf("IsDocker() = false, want true")
			}

			if got.Host != tc.wantContainer {
				t.Errorf("Container: got %q, want %q", got.Host, tc.wantContainer)
			}

			if got.RemotePath != tc.wantPath {
				t.Errorf("RemotePath: got %q, want %q", got.RemotePath, tc.wantPath)
			}
		})
	}
}

func TestParse_Errors(t *testing.T) {
	t.Parallel()

	cases := []string{
		"",
		"nopath",
		"host/path",
		"docker://",
		"docker:///only-slash",
		"@host:/path",
	}

	for _, input := range cases {
		input := input
		t.Run(input, func(t *testing.T) {
			t.Parallel()

			if _, err := target.Parse(input); err == nil {
				t.Errorf("Parse(%q) expected error, got nil", input)
			}
		})
	}
}

func TestTarget_Addr(t *testing.T) {
	t.Parallel()

	tgt, err := target.Parse("root@10.0.0.1:2222:/tmp/f")
	if err != nil {
		t.Fatal(err)
	}

	want := "10.0.0.1:2222"
	if got := tgt.Addr(); got != want {
		t.Errorf("Addr() = %q, want %q", got, want)
	}
}

func TestTarget_String(t *testing.T) {
	t.Parallel()

	t.Run("ssh", func(t *testing.T) {
		t.Parallel()

		tgt, _ := target.Parse("root@host:/path")
		s := tgt.String()

		if s == "" {
			t.Error("String() returned empty")
		}
	})

	t.Run("docker", func(t *testing.T) {
		t.Parallel()

		tgt, _ := target.Parse("docker://container:/path")
		s := tgt.String()

		if s == "" {
			t.Error("String() returned empty")
		}
	})
}

func BenchmarkParse(b *testing.B) {
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = target.Parse("deploy@prod.example.com:2222:/etc/nginx/nginx.conf")
	}
}

func BenchmarkParseDocker(b *testing.B) {
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = target.Parse("docker://my-container:/app/config.json")
	}
}
