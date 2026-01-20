package git

import "testing"

func TestParseGitRemoteURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		wantHost string
		wantOwner string
		wantRepo string
		wantNil  bool
	}{
		{
			name:      "SSH format with .git",
			url:       "git@github.com:dadlerj/tin.git",
			wantHost:  "github.com",
			wantOwner: "dadlerj",
			wantRepo:  "tin",
		},
		{
			name:      "SSH format without .git",
			url:       "git@github.com:dadlerj/tin",
			wantHost:  "github.com",
			wantOwner: "dadlerj",
			wantRepo:  "tin",
		},
		{
			name:      "HTTPS format with .git",
			url:       "https://github.com/sestinj/tin.git",
			wantHost:  "github.com",
			wantOwner: "dadlerj",
			wantRepo:  "tin",
		},
		{
			name:      "HTTPS format without .git",
			url:       "https://github.com/sestinj/tin",
			wantHost:  "github.com",
			wantOwner: "dadlerj",
			wantRepo:  "tin",
		},
		{
			name:    "invalid URL",
			url:     "not-a-url",
			wantNil: true,
		},
		{
			name:    "empty URL",
			url:     "",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseGitRemoteURL(tt.url)
			if tt.wantNil {
				if got != nil {
					t.Errorf("ParseGitRemoteURL(%q) = %+v, want nil", tt.url, got)
				}
				return
			}
			if got == nil {
				t.Fatalf("ParseGitRemoteURL(%q) = nil, want non-nil", tt.url)
			}
			if got.Host != tt.wantHost {
				t.Errorf("Host = %q, want %q", got.Host, tt.wantHost)
			}
			if got.Owner != tt.wantOwner {
				t.Errorf("Owner = %q, want %q", got.Owner, tt.wantOwner)
			}
			if got.Repo != tt.wantRepo {
				t.Errorf("Repo = %q, want %q", got.Repo, tt.wantRepo)
			}
		})
	}
}

func TestCodeHostURL_CommitURL(t *testing.T) {
	tests := []struct {
		name     string
		codeHost *CodeHostURL
		hash     string
		want     string
	}{
		{
			name:     "GitHub commit URL",
			codeHost: &CodeHostURL{Host: "github.com", Owner: "dadlerj", Repo: "tin"},
			hash:     "abc123",
			want:     "https://github.com/sestinj/tin/commit/abc123",
		},
		{
			name:     "non-GitHub host returns empty",
			codeHost: &CodeHostURL{Host: "gitlab.com", Owner: "dadlerj", Repo: "tin"},
			hash:     "abc123",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.codeHost.CommitURL(tt.hash); got != tt.want {
				t.Errorf("CommitURL(%q) = %q, want %q", tt.hash, got, tt.want)
			}
		})
	}
}
