//go:build !darwin && !windows

package localsandbox

import (
	"context"
	"fmt"

	"github.com/jrmatherly/ai-kb/internal/localsandbox/core"
	"github.com/jrmatherly/ai-kb/internal/logger"
)

// NewBackend has no implementation outside macOS and Windows.
func NewBackend() (core.Backend, error) {
	err := fmt.Errorf("%w: only macOS is supported today", core.ErrUnsupportedPlatform)
	logger.Errorf(context.Background(), "[LocalSandbox] %v", err)
	return nil, err
}
