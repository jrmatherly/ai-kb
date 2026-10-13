package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jrmatherly/ai-kb/internal/types/interfaces"
)

// WeKnoraCloudHandler manages WeKnoraCloud credentials
type WeKnoraCloudHandler struct {
	svc interfaces.WeKnoraCloudService
}

// NewWeKnoraCloudHandler creates a WeKnoraCloudHandler
func NewWeKnoraCloudHandler(svc interfaces.WeKnoraCloudService) *WeKnoraCloudHandler {
	return &WeKnoraCloudHandler{svc: svc}
}

type weKnoraCloudCredentialsRequest struct {
	AppID     string `json:"app_id"     binding:"required"`
	AppSecret string `json:"app_secret" binding:"required"`
}

// SaveCredentials POST /api/v1/weknoracloud/credentials
// Saves only the APPID/APPSECRET credentials to the workspace config; no models are created
//
// SaveCredentials godoc
// @Summary      Save WeKnoraCloud credentials
// @Description  Saves APPID/APPSECRET to the current workspace config (no models are created)
// @Tags         WeKnoraCloud
// @Accept       json
// @Produce      json
// @Param        request  body      map[string]interface{}  true  "{app_id, app_secret}"
// @Success      200      {object}  map[string]interface{}  "success: true"
// @Failure      400      {object}  map[string]interface{}  "Invalid request parameters"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /weknoracloud/credentials [post]
func (h *WeKnoraCloudHandler) SaveCredentials(c *gin.Context) {
	var req weKnoraCloudCredentialsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.svc.SaveCredentials(c.Request.Context(), req.AppID, req.AppSecret); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Credentials saved"})
}

// Status GET /api/v1/models/weknoracloud/status
// Returns needs_reinit=true when the current workspace's WeKnoraCloud credentials must be saved again
//
// Status godoc
// @Summary      Check WeKnoraCloud credential status
// @Description  Checks whether the current workspace's WeKnoraCloud credentials are intact; needs_reinit=true means
// @Description  they must be saved again
// @Tags         WeKnoraCloud
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "Credential status"
// @Failure      500  {object}  map[string]interface{}  "Internal server error"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /models/weknoracloud/status [get]
func (h *WeKnoraCloudHandler) Status(c *gin.Context) {
	result, err := h.svc.CheckStatus(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}
