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
	failure       error
}

// NewFakeConfiguration returns a FakeConfiguration that returns the configuration.
func NewFakeConfiguration(configuration dtos.Configuration) *FakeConfiguration {
	return &FakeConfiguration{configuration: configuration}
}

// FailWith makes Configuration return the error from now on. A nil error makes it work again.
func (fake_configuration *FakeConfiguration) FailWith(failure error) {
	fake_configuration.mutex.Lock()
	defer fake_configuration.mutex.Unlock()
	fake_configuration.failure = failure
}

// Configuration returns the configuration the fake was given.
func (fake_configuration *FakeConfiguration) Configuration(_ context.Context) (dtos.Configuration, error) {
	fake_configuration.mutex.Lock()
	defer fake_configuration.mutex.Unlock()
	if fake_configuration.failure != nil {
		return dtos.Configuration{}, fake_configuration.failure
	}
	return fake_configuration.configuration, nil
}
