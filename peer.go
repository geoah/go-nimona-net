package net

// Peer -
type Peer interface {
	GetID() string
	GetAddresses() []string
}
