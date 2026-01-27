package detector

import (
	"github.com/kevinelliott/agentmanager/pkg/detector/strategies"
	"github.com/kevinelliott/agentmanager/pkg/platform"
)

// NewBinaryStrategy creates a new binary detection strategy.
func NewBinaryStrategy(p platform.Platform) Strategy {
	return strategies.NewBinaryStrategy(p)
}

// NewNPMStrategy creates a new NPM detection strategy.
func NewNPMStrategy(p platform.Platform) Strategy {
	return strategies.NewNPMStrategy(p)
}

// NewPipStrategy creates a new pip/pipx/uv detection strategy.
func NewPipStrategy(p platform.Platform) Strategy {
	return strategies.NewPipStrategy(p)
}

// NewBrewStrategy creates a new Homebrew detection strategy.
func NewBrewStrategy(p platform.Platform) Strategy {
	return strategies.NewBrewStrategy(p)
}
