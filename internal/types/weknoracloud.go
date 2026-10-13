package types

// WeKnoraCloudStatusResult is a credential status check result
type WeKnoraCloudStatusResult struct {
	HasModels   bool   `json:"has_models"`       // whether WeKnoraCloud credentials are configured
	NeedsReinit bool   `json:"needs_reinit"`     // whether reinitialization is needed (broken credentials)
	Reason      string `json:"reason,omitempty"` // why reinitialization is needed
}
