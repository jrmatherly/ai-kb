// Package runtime provides the application's dependency injection container,
// built on uber's dig library.
package runtime

import (
	"go.uber.org/dig"
)

// container is the application's global dependency injection container;
// every service and component is registered and resolved through it.
var container *dig.Container

// init creates the dependency injection container
// at program start.
func init() {
	container = dig.New()
}

// GetContainer returns the global dependency injection container
// for other packages to register or resolve services.
func GetContainer() *dig.Container {
	return container
}
