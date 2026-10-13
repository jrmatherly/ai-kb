package handler

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	apperrors "github.com/jrmatherly/ai-kb/internal/errors"
)

// hostProjectPicker opens the native folder dialog on Lite desktop.
// Nil on every other deployment: the route then 404s.
var (
	hostProjectPickerMu sync.RWMutex
	hostProjectPicker   func() (string, error)
)

// SetHostProjectPicker is called by cmd/desktop. Standard servers leave it nil.
func SetHostProjectPicker(fn func() (string, error)) {
	hostProjectPickerMu.Lock()
	hostProjectPicker = fn
	hostProjectPickerMu.Unlock()
}

func currentHostProjectPicker() func() (string, error) {
	hostProjectPickerMu.RLock()
	defer hostProjectPickerMu.RUnlock()
	return hostProjectPicker
}

// PickHostProjectDir opens the system folder picker. The SPA talks HTTP to
// this process; Wails JS bindings are not injected through the
// reverse-proxied frontend, so this is the working authorization path.
func (*SystemHandler) PickHostProjectDir(c *gin.Context) {
	picker := currentHostProjectPicker()
	if picker == nil {
		_ = c.Error(apperrors.NewNotFoundError("host project picker is not available"))
		return
	}
	dir, err := picker()
	if err != nil {
		_ = c.Error(apperrors.NewInternalServerError(err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": gin.H{"dir": dir},
	})
}
