package xlog

import (
	"github.com/go-kit/kit/log"
	"github.com/spf13/viper"
)

// Unmarshal loads an Options from a Viper instance and produces a go-kit Logger
func Unmarshal(key string, v *viper.Viper) (log.Logger, error) {
	var o Options
	if err := v.UnmarshalKey(key, &o); err != nil {
		return nil, err
	}

	return New(o)
}
