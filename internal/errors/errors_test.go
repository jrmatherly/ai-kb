package errors

import (
	"fmt"
	"testing"
)

func TestIsAppErrorFindsWrappedAppError(t *testing.T) {
	inner := NewNotFoundError("thing not found")

	got, ok := IsAppError(fmt.Errorf("lookup: %w", inner))

	if !ok {
		t.Fatal("IsAppError did not find the wrapped AppError")
	}
	if got != inner {
		t.Fatalf("IsAppError returned %p, want the wrapped pointer %p", got, inner)
	}
}
