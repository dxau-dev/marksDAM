package cmd

import (
	"testing"
)

func TestBuildImageURL(t *testing.T) {
	tests := []struct {
		name       string
		webHost    string
		webURLPath string
		imagePath  string
		want       string
	}{
		{
			name:       "host with trailing slash",
			webHost:    "https://example.com/",
			webURLPath: "photos",
			imagePath:  "img.jpg",
			want:       "https://example.com/photos/img.jpg",
		},
		{
			name:       "host without trailing slash gets one added",
			webHost:    "https://example.com",
			webURLPath: "photos",
			imagePath:  "img.jpg",
			want:       "https://example.com/photos/img.jpg",
		},
		{
			name:       "empty web url path",
			webHost:    "https://example.com/",
			webURLPath: "",
			imagePath:  "img.jpg",
			want:       "https://example.com/img.jpg",
		},
		{
			name:       "dotslash prefix stripped from image path",
			webHost:    "https://example.com/",
			webURLPath: "gallery",
			imagePath:  "./subdir/img.jpg",
			want:       "https://example.com/gallery/subdir/img.jpg",
		},
		{
			name:       "nested image path",
			webHost:    "https://example.com/",
			webURLPath: "gallery/2024",
			imagePath:  "january/party.jpg",
			want:       "https://example.com/gallery/2024/january/party.jpg",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildImageURL(tt.webHost, tt.webURLPath, tt.imagePath)
			if got != tt.want {
				t.Errorf("buildImageURL() = %q, want %q", got, tt.want)
			}
		})
	}
}
