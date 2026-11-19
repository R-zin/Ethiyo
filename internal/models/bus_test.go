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

func TestParseBusCodes(t *testing.T) {
	got, err := ParseBusCodes(" KS602, DL1PC0001 ,,KS602, route_500 ", 5)
	if err != nil {
		t.Fatalf("ParseBusCodes: %v", err)
	}
	want := []string{"KS602", "DL1PC0001", "route_500"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %q, want %q (trimmed, de-duplicated, order kept)", i, got[i], want[i])
		}
	}
}

func TestParseBusCodesErrors(t *testing.T) {
	if _, err := ParseBusCodes(" , ,", 5); !errors.Is(err, ErrBusCodeRequired) {
		t.Errorf("empty list: err = %v, want ErrBusCodeRequired", err)
	}
	_, err := ParseBusCodes("KS602,bad$code", 5)
	if !errors.Is(err, ErrInvalidBusCodeFormat) || !strings.Contains(err.Error(), "entry 2") {
		t.Errorf("invalid entry: err = %v, want ErrInvalidBusCodeFormat naming entry 2", err)
	}
	if _, err := ParseBusCodes("A,B,C", 2); !errors.Is(err, ErrTooManyBusCodes) {
		t.Errorf("too many: err = %v, want ErrTooManyBusCodes", err)
	}
	// Duplicates do not count toward the limit.
	if codes, err := ParseBusCodes("A,A,A,B", 2); err != nil || len(codes) != 2 {
		t.Errorf("duplicates: codes=%v err=%v", codes, err)
	}
}
