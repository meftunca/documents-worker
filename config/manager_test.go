package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigManager(t *testing.T) {
	t.Run("create new manager", func(t *testing.T) {
		manager := NewManager("development")
		assert.NotNil(t, manager)
		assert.Equal(t, "development", manager.environment)
	})

	t.Run("load from environment", func(t *testing.T) {
		manager := NewManager("test")
		err := manager.LoadFromEnv()
		require.NoError(t, err)

		config := manager.GetConfig()
		assert.NotNil(t, config)
		assert.Equal(t, "8080", config.Server.Port)
	})

	t.Run("feature flags", func(t *testing.T) {
		manager := NewManager("test")

		// Initially no flags
		assert.False(t, manager.IsFeatureEnabled("new_feature"))

		// Set a feature flag
		manager.SetFeatureFlag("new_feature", true)
		assert.True(t, manager.IsFeatureEnabled("new_feature"))

		// Disable feature flag
		manager.SetFeatureFlag("new_feature", false)
		assert.False(t, manager.IsFeatureEnabled("new_feature"))
	})
}
