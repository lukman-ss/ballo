package ballo

// ServiceProvider is implemented by any package that wants to integrate with Ballo.
// Register is called during App.Register; Boot is called after all providers are registered.
//
// Packages such as ballo-auth, ballo-session, ballo-render, and ballo-migrate should
// implement this interface and pass their provider to App.Register.
type ServiceProvider interface {
	// Register binds services into the container. Do not resolve other services here.
	Register(app *App)
	// Boot runs after all providers are registered. Safe to resolve dependencies here.
	Boot(app *App)
}
