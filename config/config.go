package config

import (
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	// raft store config
	RetainSnapshotCount int           `mapstructure:"RETAIN_SNAPSHOT_COUNT"`
	RaftTimeout         time.Duration `mapstructure:"RAFT_TIMEOUT"`
	LeaderWaitDelay     time.Duration `mapstructure:"LEADER_WAIT_DELAY"`
	AppliedWaitDelay    time.Duration `mapstructure:"APPLIED_WAIT_DELAY"`
	TransportMaxPool    int           `mapstructure:"TRANSPORT_MAX_POOL"`
	TransportTimeout    time.Duration `mapstructure:"TRANSPORT_TIMEOUT"`

	// raft server config
	OpenTimeout time.Duration `mapstructure:"OPEN_TIMEOUT"`
}

func LoadConfig(path string) (config Config, err error) {
	viper.AddConfigPath(path)
	viper.SetConfigName("app")
	viper.SetConfigType("env")

	viper.AutomaticEnv()

	if err = viper.ReadInConfig(); err != nil {
		return
	}
	err = viper.Unmarshal(&config)
	return

}
