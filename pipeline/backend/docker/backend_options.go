package docker

import (
	"github.com/go-viper/mapstructure/v2"

	backend "codefloe.com/crowci/crow/v6/pipeline/backend/types"
)

// BackendOptions defines all the advanced options for the docker backend.
type BackendOptions struct {
	User string `mapstructure:"user"`
	// Isolation raises a Windows container to Hyper-V isolation. It can only
	// strengthen the agent's own setting, never relax it. See resolveIsolation.
	Isolation string `mapstructure:"isolation"`
}

func parseBackendOptions(step *backend.Step) (BackendOptions, error) {
	var result BackendOptions
	if step == nil || step.BackendOptions == nil {
		return result, nil
	}
	err := mapstructure.WeakDecode(step.BackendOptions[EngineName], &result)
	return result, err
}
