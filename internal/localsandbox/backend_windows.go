//go:build windows

package localsandbox

import (
	"github.com/jrmatherly/ai-kb/internal/localsandbox/core"
	"github.com/jrmatherly/ai-kb/internal/localsandbox/winhost"
)

func NewBackend() (core.Backend, error) { return winhost.New() }
