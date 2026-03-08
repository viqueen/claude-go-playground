package main

// Domains holds all domain services. Fields are added by the integrate agent.
type Domains struct{}

func setupDomains(_ *Connections) *Domains {
	return &Domains{}
}
