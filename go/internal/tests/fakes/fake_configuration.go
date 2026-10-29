package fakes

import (
	"context"
	"sync"

	"github.com/DanielCalvo-Calvicia/telemetry/go/internal/application/dtos"
)

// FakeConfiguration is a fake of outbound.ConfigurationPort. It returns the configuration it was given.
type FakeConfiguration struct {
	mutex         sync.Mutex
	configuration dtos.Configuration
}

// NewFakeConfiguration returns a FakeConfiguration that returns the configuration.
func NewFakeConfiguration(configuration dtos.Configuration) *FakeConfiguration {
	return &FakeConfiguration{configuration: configuration}
}

// Configuration returns the configuration the fake was given.
func (fake_configuration *FakeConfiguration) Configuration(_ context.Context) (dtos.Configuration, error) {
	fake_configuration.mutex.Lock()
	defer fake_configuration.mutex.Unlock()
	return fake_configuration.configuration, nil
}
