package im

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnabledPlatforms_UnsetMeansAll(t *testing.T) {
	t.Setenv(PlatformsEnv, "")
	require.Equal(t, AllPlatforms(), EnabledPlatforms())
	assert.Equal(t, []Platform{
		PlatformWeCom, PlatformFeishu, PlatformLark, PlatformSlack, PlatformTelegram,
		PlatformDingtalk, PlatformMattermost, PlatformWeChat, PlatformQQBot, PlatformYunzhijia,
	}, AllPlatforms())
}

func TestEnabledPlatforms_ParsesLikeStorageAllowList(t *testing.T) {
	t.Setenv(PlatformsEnv, "slack; Telegram foo")
	assert.Equal(t, []Platform{PlatformSlack, PlatformTelegram}, EnabledPlatforms())
	assert.True(t, PlatformEnabled(PlatformSlack))
	assert.False(t, PlatformEnabled(PlatformWeCom))
}

func TestEnabledPlatforms_BlankMeansAllOnlyUnknownMeansNone(t *testing.T) {
	t.Setenv(PlatformsEnv, " ")
	assert.Equal(t, AllPlatforms(), EnabledPlatforms())
	t.Setenv(PlatformsEnv, "foo")
	assert.Empty(t, EnabledPlatforms())
	assert.False(t, PlatformEnabled(PlatformSlack))
}

func TestRegisterAdapterFactory_SkipsDisabledPlatform(t *testing.T) {
	t.Setenv(PlatformsEnv, "slack")
	s := &Service{
		adapterFactories: make(map[string]AdapterFactory),
		channels:         make(map[string]*channelState),
		leaderRetries:    make(map[string]*leaderRetryState),
	}
	s.RegisterAdapterFactory("wecom", nil)
	s.RegisterAdapterFactory("slack", nil)
	_, hasWeCom := s.adapterFactories["wecom"]
	_, hasSlack := s.adapterFactories["slack"]
	assert.False(t, hasWeCom, "a disabled platform must not get a factory")
	assert.True(t, hasSlack)

	err := s.StartChannel(&IMChannel{ID: "ch-1", Platform: "wecom", Mode: "webhook"})
	require.ErrorContains(t, err, "no adapter factory for platform: wecom")
}
