package http

import (
	"io"
	"net/http"
	"strings"
	"testing"

	cmds "github.com/ipfs/go-ipfs-cmds"
)

func TestSetEncodingType(t *testing.T) {
	testCases := []struct {
		name                string
		path                string
		expectedContentType string
	}{
		{
			name:                "octet stream sets application/octet-stream",
			path:                "/octetstream",
			expectedContentType: "application/octet-stream",
		},
		{
			name:                "custom content type overrides encoding",
			path:                "/customcontenttype",
			expectedContentType: "application/vnd.ipld.car",
		},
		{
			name:                "reader without SetEncodingType defaults to text/plain",
			path:                "/reader",
			expectedContentType: "text/plain",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, srv := getTestServer(t, nil, true)
			defer srv.Close()

			req, err := http.NewRequest("POST", srv.URL+tc.path, nil)
			if err != nil {
				t.Fatal(err)
			}
			// Origin header is required by CORS validation
			req.Header.Set("Origin", "http://localhost")

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()

			// Drain the body
			_, _ = io.Copy(io.Discard, resp.Body)

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("unexpected status code: %d", resp.StatusCode)
			}

			// Content-Type may include parameters like "; charset=utf-8"
			contentType := resp.Header.Get("Content-Type")
			contentType = strings.Split(contentType, ";")[0]
			if contentType != tc.expectedContentType {
				t.Errorf("expected Content-Type %q, got %q", tc.expectedContentType, contentType)
			}
		})
	}
}

// TestSetEncodingTypeBackwardCompatibility verifies that when SetEncodingType
// is NOT called, the Content-Type header behaves exactly as before:
//   - io.Reader with default encoding returns text/plain (for security, to
//     avoid browsers rendering HTML from untrusted content)
//   - Structured data uses the requested encoding's MIME type
func TestSetEncodingTypeBackwardCompatibility(t *testing.T) {
	_, srv := getTestServer(t, nil, true)
	defer srv.Close()

	// Test that "reader" command (which doesn't call SetEncodingType)
	// still returns text/plain for backward compatibility.
	// This is important for security: prevents browsers from rendering
	// HTML content when accessing the API directly.
	req, err := http.NewRequest("POST", srv.URL+"/reader", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Origin", "http://localhost")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status code: %d", resp.StatusCode)
	}

	contentType := strings.Split(resp.Header.Get("Content-Type"), ";")[0]
	if contentType != "text/plain" {
		t.Errorf("backward compatibility broken: expected Content-Type %q for reader, got %q",
			"text/plain", contentType)
	}

	// Test that "version" command (structured JSON output) still returns application/json
	req2, err := http.NewRequest("POST", srv.URL+"/version", nil)
	if err != nil {
		t.Fatal(err)
	}
	req2.Header.Set("Origin", "http://localhost")

	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	_, _ = io.Copy(io.Discard, resp2.Body)

	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status code: %d", resp2.StatusCode)
	}

	contentType2 := strings.Split(resp2.Header.Get("Content-Type"), ";")[0]
	if contentType2 != "application/json" {
		t.Errorf("backward compatibility broken: expected Content-Type %q for version, got %q",
			"application/json", contentType2)
	}
}

// TestMIMEEncodingsMapping verifies the MIMEEncodings map correctly maps
// MIME types to encoding types for response parsing
func TestMIMEEncodingsMapping(t *testing.T) {
	tests := []struct {
		mimeType string
		encType  cmds.EncodingType
	}{
		{"application/json", cmds.JSON},
		{"application/xml", cmds.XML},
		{"text/plain", cmds.Text},
		{"application/octet-stream", cmds.OctetStream},
	}

	for _, tt := range tests {
		t.Run(tt.mimeType, func(t *testing.T) {
			got, ok := MIMEEncodings[tt.mimeType]
			if !ok {
				t.Errorf("MIMEEncodings missing entry for %q", tt.mimeType)
				return
			}
			if got != tt.encType {
				t.Errorf("MIMEEncodings[%q] = %q, want %q", tt.mimeType, got, tt.encType)
			}
		})
	}
}
