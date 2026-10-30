package config

import "github.com/DanielCalvo-Calvicia/telemetry/go/internal/domain/value_objects"

// =============================================================================
// PROFILES
// =============================================================================

// The profiles are only defaults (REQ-CFG-004): each one is Defaults with the delivery mode that fits the kind of
// process, and nothing else changes. The consumer overrides any field after choosing one, so the precedence, lowest to
// highest, is: the defaults, the profile, the options given in code.

// with_delivery_mode returns the defaults with the delivery mode.
func with_delivery_mode(delivery_mode value_objects.DeliveryMode) Config {
	config := Defaults()
	config.Delivery.Mode = string(delivery_mode)
	return config
}

// Microservice returns the defaults for a long-running service: the delivery mode is best_effort.
func Microservice() Config {
	return with_delivery_mode(value_objects.DeliveryModeBestEffort)
}

// Cronjob returns the defaults for a short scheduled job: the delivery mode is retry.
func Cronjob() Config {
	return with_delivery_mode(value_objects.DeliveryModeRetry)
}

// Worker returns the defaults for a background worker: the delivery mode is retry.
func Worker() Config {
	return with_delivery_mode(value_objects.DeliveryModeRetry)
}

// Robotics returns the defaults for a robot, which may lose the network: the delivery mode is durable, so the consumer
// must also give Spool.Directory.
func Robotics() Config {
	return with_delivery_mode(value_objects.DeliveryModeDurable)
}

// OfflineDevice returns the defaults for a device that is often offline: the delivery mode is durable, so the
// consumer must also give Spool.Directory.
func OfflineDevice() Config {
	return with_delivery_mode(value_objects.DeliveryModeDurable)
}
