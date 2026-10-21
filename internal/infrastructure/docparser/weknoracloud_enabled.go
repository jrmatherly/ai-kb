package docparser

import "github.com/jrmatherly/ai-kb/internal/envlist"

// WeKnoraCloudEnabledEnv opts a deployment into the hosted WeKnoraCloud
// parser and model provider. Off by default: it is a client for Tencent's
// hosted service, and documents sent through it leave the deployment.
const WeKnoraCloudEnabledEnv = "AIKB_WEKNORACLOUD_ENABLED"

// WeKnoraCloudEnabled is read at call time, never in init(): the desktop
// binary loads .env after package initialization.
func WeKnoraCloudEnabled() bool {
	return envlist.Bool(WeKnoraCloudEnabledEnv)
}

// WeKnoraCloudDisabledReason is the user-facing reason every gated surface
// reports while the deployment has not opted in.
const WeKnoraCloudDisabledReason = "disabled by this deployment; set " + WeKnoraCloudEnabledEnv + "=true to enable"
