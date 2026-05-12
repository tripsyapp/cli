package coverimage

import "testing"

func TestValidateDirectUnsplashURL(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr bool
	}{
		{
			name: "direct numeric photo asset",
			raw:  "https://images.unsplash.com/photo-1562869929-bda0650edb1f?ixid=M3w4NDY3MXwwfDF8YWxsfHx8fHx8fHx8MTc3ODYxOTg5Nnw&ixlib=rb-4.1.0",
		},
		{
			name:    "short image id",
			raw:     "https://images.unsplash.com/photo-nWdsya5_Yms?ixlib=rb-4.1.0",
			wantErr: true,
		},
		{
			name:    "unsplash page url",
			raw:     "https://unsplash.com/photos/nWdsya5_Yms",
			wantErr: true,
		},
		{
			name:    "non photo asset path",
			raw:     "https://images.unsplash.com/flagged/photo-1562869929-bda0650edb1f",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDirectUnsplashURL(tt.raw)
			if tt.wantErr && err == nil {
				t.Fatal("ValidateDirectUnsplashURL() error = nil, want error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("ValidateDirectUnsplashURL() error = %v, want nil", err)
			}
		})
	}
}
