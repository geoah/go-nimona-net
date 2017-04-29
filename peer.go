package net

// Peer -
type Peer interface {
	GetID() string
	GetAddresses() []string
	// Handle(ev *Event) error // TODO Temp, split peer from handler
}
