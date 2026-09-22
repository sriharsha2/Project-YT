package apperr

import (
	"errors"
	"fmt"
	"testing"
)

func TestConstructorsSetKindAndMessage(t *testing.T) {
	cases := map[string]struct {
		err  *Error
		kind Kind
	}{
		"validation": {Validation("title is required"), KindValidation},
		"not found":  {NotFound("video not found"), KindNotFound},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if tc.err.Kind != tc.kind {
				t.Fatalf("kind = %v, want %v", tc.err.Kind, tc.kind)
			}
			if tc.err.Error() != tc.err.Message {
				t.Fatalf("Error() = %q, want message %q", tc.err.Error(), tc.err.Message)
			}
		})
	}
}

func TestErrorIncludesAndUnwrapsCause(t *testing.T) {
	cause := errors.New("connection reset")
	err := &Error{Kind: KindInternal, Message: "load video", Cause: cause}

	if err.Error() != "load video: connection reset" {
		t.Fatalf("Error() = %q", err.Error())
	}
	if !errors.Is(err, cause) {
		t.Fatal("expected errors.Is to find the cause")
	}
}

func TestAsFindsWrappedError(t *testing.T) {
	original := NotFound("video not found")
	wrapped := fmt.Errorf("handler: %w", original)

	got, ok := As(wrapped)

	if !ok || got != original {
		t.Fatalf("As() = %v, %v", got, ok)
	}
}

func TestAsReportsUnclassifiedError(t *testing.T) {
	got, ok := As(errors.New("boom"))

	if ok || got != nil {
		t.Fatalf("As() = %v, %v; want nil, false", got, ok)
	}
}
