package main

import "sync"

// Server holds data about a guild's channels and other things
type Server struct {
	channels        map[string]*Channel
	orderedChannels []Channel
	// Parent of all the channels
	category string
	// Prefix for the name of the channels to be created
	prefix      string
	initialized bool
	mutex       *sync.Mutex
}

// Channel holds info about a channel
type Channel struct {
	name            string
	id              string
	connectedPeople int
}
