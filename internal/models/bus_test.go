package models

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestValidateBusCode(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr error
	}{
		{"valid digits", "12345", "12345", nil},
		{"valid alphanumeric", "DL1PC0001", "DL1PC0001", nil},
		{"valid with hyphen and underscore", "route_500-A", "route_500-A", nil},
		{"valid with leading/trailing whitespace", "  KA01F1234  ", "KA01F1234", nil},
		{"empty string", "", "", ErrBusCodeRequired},
		{"whitespace only", "   ", "", ErrBusCodeRequired},
		{"path traversal", "../etc/passwd", "", ErrInvalidBusCodeFormat},
		{"query injection", "12345?foo=bar", "", ErrInvalidBusCodeFormat},
		{"slashes", "route/123", "", ErrInvalidBusCodeFormat},
		{"special chars", "route$!#", "", ErrInvalidBusCodeFormat},
		{"exceeds max length", strings.Repeat("a", 65), "", ErrInvalidBusCodeFormat},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateBusCode(tt.input)
			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected error %v, got %v", tt.wantErr, err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got != tt.want {
					t.Errorf("expected %q, got %q", tt.want, got)
				}
			}
		})
	}
}

func TestValidateBusCodeBoundaryLengths(t *testing.T) {
	if _, err := ValidateBusCode(strings.Repeat("a", 64)); err != nil {
		t.Errorf("64-char code should be valid: %v", err)
	}
	if _, err := ValidateBusCode(strings.Repeat("a", 65)); !errors.Is(err, ErrInvalidBusCodeFormat) {
		t.Errorf("65-char code should be invalid, got %v", err)
	}
}

func TestResponseSerialization(t *testing.T) {
	errResp := NewErrorResponse("TEST_CODE", "test error message")
	if errResp.Success {
		t.Errorf("expected Success to be false")
	}

	data, err := json.Marshal(errResp)
	if err != nil {
		t.Fatalf("failed to marshal ErrorResponse: %v", err)
	}

	jsonStr := string(data)
	if !strings.Contains(jsonStr, `"success":false`) {
		t.Errorf("expected json to contain explicit '\"success\":false', got: %s", jsonStr)
	}

	succResp := NewSuccessResponse(map[string]string{"foo": "bar"})
	if !succResp.Success {
		t.Errorf("expected Success to be true")
	}

	succData, err := json.Marshal(succResp)
	if err != nil {
		t.Fatalf("failed to marshal SuccessResponse: %v", err)
	}
	if !strings.Contains(string(succData), `"success":true`) {
		t.Errorf("expected json to contain '\"success\":true', got: %s", string(succData))
	}
}
