package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	configPath := "../"
	config, err := LoadConfig(configPath)
	require.NoError(t, err)
	require.NotEmpty(t, config)

	require.Equal(t, 2, config.RetainSnapshotCount)
	require.Equal(t, 10*time.Second, config.RaftTimeout)
	require.Equal(t, 100*time.Millisecond, config.AppliedWaitDelay)
	require.Equal(t, 100*time.Millisecond, config.LeaderWaitDelay)
	require.Equal(t, 3, config.TransportMaxPool)
	require.Equal(t, 10*time.Second, config.TransportTimeout)
	require.Equal(t, 120*time.Second, config.OpenTimeout)
}
