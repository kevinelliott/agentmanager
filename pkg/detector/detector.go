// Package detector provides agent detection capabilities.
package detector

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/kevinelliott/agentmanager/pkg/agent"
	"github.com/kevinelliott/agentmanager/pkg/catalog"
	"github.com/kevinelliott/agentmanager/pkg/platform"
)

// Strategy defines how to detect agents installed via a specific method.
type Strategy interface {
	// Name returns the strategy name (e.g., "npm", "brew", "pip").
	Name() string

	// Method returns the install method this strategy detects.
	Method() agent.InstallMethod

	// IsApplicable returns true if this strategy can run on the given platform.
	IsApplicable(p platform.Platform) bool

	// Detect scans for installed agents and returns found installations.
	Detect(ctx context.Context, agents []catalog.AgentDef) ([]*agent.Installation, error)
}

// Detector orchestrates agent detection across multiple strategies.
type Detector struct {
	strategies     []Strategy
	platform       platform.Platform
	pluginRegistry *PluginRegistry
	mu             sync.RWMutex
}

// New creates a new Detector with all available strategies.
func New(p platform.Platform) *Detector {
	d := &Detector{
		platform:       p,
		strategies:     make([]Strategy, 0),
		pluginRegistry: NewPluginRegistry(p),
	}

	// Register default strategies
	d.RegisterStrategy(NewBinaryStrategy(p))
	d.RegisterStrategy(NewNPMStrategy(p))
	d.RegisterStrategy(NewPipStrategy(p))
	d.RegisterStrategy(NewBrewStrategy(p))

	return d
}

// PluginRegistry returns the detector's plugin registry.
func (d *Detector) PluginRegistry() *PluginRegistry {
	return d.pluginRegistry
}

// LoadPlugins loads detection plugins from the given directory.
func (d *Detector) LoadPlugins(dir string) error {
	if err := d.pluginRegistry.LoadPluginsFromDir(dir); err != nil {
		return err
	}

	// Register plugin strategies
	for _, s := range d.pluginRegistry.GetStrategies() {
		d.RegisterStrategy(s)
	}

	return nil
}

// RegisterStrategy adds a detection strategy.
func (d *Detector) RegisterStrategy(s Strategy) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.strategies = append(d.strategies, s)
}

// GetStrategies returns all registered strategies.
func (d *Detector) GetStrategies() []Strategy {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.strategies
}

// DetectAll runs all applicable strategies and returns found installations.
func (d *Detector) DetectAll(ctx context.Context, agents []catalog.AgentDef) ([]*agent.Installation, error) {
	d.mu.RLock()
	strategies := d.strategies
	d.mu.RUnlock()

	var wg sync.WaitGroup
	resultsChan := make(chan []*agent.Installation, len(strategies))
	errorsChan := make(chan error, len(strategies))

	for _, s := range strategies {
		if !s.IsApplicable(d.platform) {
			continue
		}

		wg.Add(1)
		go func(strategy Strategy) {
			defer wg.Done()

			installations, err := strategy.Detect(ctx, agents)
			if err != nil {
				errorsChan <- fmt.Errorf("%s detection failed: %w", strategy.Name(), err)
				return
			}

			if len(installations) > 0 {
				resultsChan <- installations
			}
		}(s)
	}

	// Wait for all strategies to complete
	wg.Wait()
	close(resultsChan)
	close(errorsChan)

	// Collect all results
	var allInstallations []*agent.Installation
	for installations := range resultsChan {
		allInstallations = append(allInstallations, installations...)
	}

	// Drain error channel (errors are logged by strategies, not surfaced)
	for range errorsChan {
		// Errors from individual strategies are logged but don't fail detection
	}

	// Deduplicate installations by key
	allInstallations = deduplicateInstallations(allInstallations)

	// Set detection timestamp
	now := time.Now()
	for _, inst := range allInstallations {
		if inst.DetectedAt.IsZero() {
			inst.DetectedAt = now
		}
		inst.LastChecked = now
	}

	return allInstallations, nil
}

// DetectByMethod runs only the strategy for the given method.
func (d *Detector) DetectByMethod(ctx context.Context, method agent.InstallMethod, agents []catalog.AgentDef) ([]*agent.Installation, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	for _, s := range d.strategies {
		if s.Method() == method && s.IsApplicable(d.platform) {
			installations, err := s.Detect(ctx, agents)
			if err != nil {
				return nil, fmt.Errorf("%s detection failed: %w", s.Name(), err)
			}

			// Set timestamps
			now := time.Now()
			for _, inst := range installations {
				if inst.DetectedAt.IsZero() {
					inst.DetectedAt = now
				}
				inst.LastChecked = now
			}

			return installations, nil
		}
	}

	return nil, fmt.Errorf("no strategy available for method: %s", method)
}

// DetectAgent detects a specific agent using all applicable strategies.
func (d *Detector) DetectAgent(ctx context.Context, agentDef catalog.AgentDef) ([]*agent.Installation, error) {
	return d.DetectAll(ctx, []catalog.AgentDef{agentDef})
}

// deduplicateInstallations removes duplicate installations by key.
func deduplicateInstallations(installations []*agent.Installation) []*agent.Installation {
	seen := make(map[string]bool)
	var result []*agent.Installation

	for _, inst := range installations {
		key := inst.Key()
		if !seen[key] {
			seen[key] = true
			result = append(result, inst)
		}
	}

	return result
}

// Result represents the result of a detection run.
type Result struct {
	Installations []*agent.Installation
	Errors        []error
	Duration      time.Duration
}

// NewInstallations returns installations that are not in the existing list.
func (r *Result) NewInstallations(existing []*agent.Installation) []*agent.Installation {
	existingKeys := make(map[string]bool)
	for _, inst := range existing {
		existingKeys[inst.Key()] = true
	}

	var newInsts []*agent.Installation
	for _, inst := range r.Installations {
		if !existingKeys[inst.Key()] {
			newInsts = append(newInsts, inst)
		}
	}

	return newInsts
}

// RemovedInstallations returns installations that exist but were not detected.
func (r *Result) RemovedInstallations(existing []*agent.Installation) []*agent.Installation {
	detectedKeys := make(map[string]bool)
	for _, inst := range r.Installations {
		detectedKeys[inst.Key()] = true
	}

	var removed []*agent.Installation
	for _, inst := range existing {
		if !detectedKeys[inst.Key()] {
			removed = append(removed, inst)
		}
	}

	return removed
}
