package main

// Removes element from the queue
func removeFromOrderedChannels(id string, guild string) {
	for i, q := range server[guild].orderedChannels {
		if q.id == id {
			copy(server[guild].orderedChannels[i:], server[guild].orderedChannels[i+1:])
			server[guild].orderedChannels[len(server[guild].orderedChannels)-1] = Channel{
				name:            "",
				id:              "",
				connectedPeople: 0,
			}
			server[guild].orderedChannels = server[guild].orderedChannels[:len(server[guild].orderedChannels)-1]
			return
		}
	}
}
