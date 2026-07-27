package psgallery

import "testing"

func TestFeedOverrides(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		apiKey   string
		wantBase string
		wantKey  string
	}{
		{
			name:     "defaults to psgallery",
			wantBase: psGalleryBaseURL,
		},
		{
			name:     "custom url overrides psgallery",
			url:      "https://nuget.example.com/v2",
			wantBase: "https://nuget.example.com/v2",
		},
		{
			name:     "custom url trailing slash trimmed",
			url:      "https://nuget.example.com/v2/",
			wantBase: "https://nuget.example.com/v2",
		},
		{
			name:     "api key honoured with custom url",
			url:      "https://nuget.example.com/v2",
			apiKey:   "secret",
			wantBase: "https://nuget.example.com/v2",
			wantKey:  "secret",
		},
		{
			name:     "api key ignored without custom url",
			apiKey:   "secret",
			wantBase: psGalleryBaseURL,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(envFeedURL, tt.url)
			t.Setenv(envFeedAPIKey, tt.apiKey)
			if got := feedBaseURL(); got != tt.wantBase {
				t.Errorf("feedBaseURL() = %q, want %q", got, tt.wantBase)
			}
			if got := feedAPIKey(); got != tt.wantKey {
				t.Errorf("feedAPIKey() = %q, want %q", got, tt.wantKey)
			}
		})
	}
}
