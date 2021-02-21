package main

import (
	"github.com/bwmarrin/lit"
	"github.com/spf13/viper"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/bwmarrin/discordgo"
)

var (
	token  string
	server = make(map[string]*Server)
)

func init() {

	lit.LogLevel = lit.LogError

	viper.SetConfigName("config")
	viper.SetConfigType("yml")
	viper.AddConfigPath(".")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found
			lit.Error("Config file not found! See example_config.yml")
			return
		}
	} else {
		// Config file found
		token = viper.GetString("token")

		// Set lit.LogLevel to the given value
		switch strings.ToLower(viper.GetString("loglevel")) {
		case "logerror", "error":
			lit.LogLevel = lit.LogError
			break
		case "logwarning", "warning":
			lit.LogLevel = lit.LogWarning
			break
		case "loginformational", "informational":
			lit.LogLevel = lit.LogInformational
			break
		case "logdebug", "debug":
			lit.LogLevel = lit.LogDebug
			break
		}

		// Populate categories
		for _, p := range strings.Split(viper.GetString("category"), ",") {
			s := strings.Split(strings.TrimSpace(p), ":")

			if len(s) == 3 {
				if server[s[0]] == nil {
					server[s[0]] = &Server{channels: make(map[string]*Channel), category: strings.TrimSpace(s[1]), prefix: strings.TrimSpace(s[2]), initialized: false, mutex: &sync.Mutex{}}
				}
			} else {
				lit.Warn("invalid format for categories: %s", strings.TrimSpace(p))
			}
		}

	}
}

func main() {

	// Create a new Discord session using the provided bot token.
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		lit.Error("error creating Discord session,", err)
		return
	}

	dg.AddHandler(voiceStateUpdate)
	dg.AddHandler(guildCreate)

	dg.Identify.Intents = discordgo.MakeIntent(discordgo.IntentsGuildVoiceStates + discordgo.IntentsGuilds)

	// Open a websocket connection to Discord and begin listening.
	err = dg.Open()
	if err != nil {
		lit.Error("error opening connection,", err)
		return
	}

	// Wait here until CTRL-C or other term signal is received.
	lit.Info("dynamicChannels is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	// Cleanly close down the Discord session.
	_ = dg.Close()
}

// Initialize Server structure
func guildCreate(s *discordgo.Session, e *discordgo.GuildCreate) {

	if server[e.ID].initialized {
		return
	}

	server[e.ID].initialized = true

	for _, g := range s.State.Guilds {
		if g.ID == e.ID {
			for _, c := range g.Channels {
				if c.ParentID == server[c.GuildID].category {
					server[c.GuildID].channels[c.ID] = &Channel{
						name:            c.Name,
						connectedPeople: 0,
						id:              c.ID,
					}
				}
			}
		}
	}

	for _, c := range server[e.ID].channels {
		server[e.ID].orderedChannels = append(server[e.ID].orderedChannels, *c)
	}

	sort.Slice(server[e.ID].orderedChannels, func(p, q int) bool {
		a, _ := strconv.Atoi(strings.TrimPrefix(server[e.ID].orderedChannels[p].name, server[e.ID].prefix+" "))
		b, _ := strconv.Atoi(strings.TrimPrefix(server[e.ID].orderedChannels[q].name, server[e.ID].prefix+" "))

		return a < b
	})
}

func voiceStateUpdate(s *discordgo.Session, v *discordgo.VoiceStateUpdate) {

	// Trace how many people are in a voice channel
	if v.BeforeUpdate == nil {
		// New connection
		if server[v.GuildID].channels[v.ChannelID] != nil {
			server[v.GuildID].channels[v.ChannelID].connectedPeople++
		}
	} else {
		if server[v.GuildID].channels[v.ChannelID] != nil {
			server[v.GuildID].channels[v.ChannelID].connectedPeople++
		}

		if server[v.GuildID].channels[v.BeforeUpdate.ChannelID] != nil && server[v.GuildID].channels[v.BeforeUpdate.ChannelID].connectedPeople > 0 {
			server[v.GuildID].channels[v.BeforeUpdate.ChannelID].connectedPeople--
		}
	}

	server[v.GuildID].mutex.Lock()

	// Create or delete channels as needed
	for i, c := range server[v.GuildID].orderedChannels {
		if server[v.GuildID].channels[c.id] == nil {
			continue
		}

		if i != 0 {
			// If it's empty, delete it
			if server[v.GuildID].channels[c.id].connectedPeople == 0 && server[v.GuildID].channels[server[v.GuildID].orderedChannels[i-1].id].connectedPeople == 0 {
				removeFromOrderedChannels(c.id, v.GuildID)
				server[v.GuildID].channels[c.id] = nil

				_, _ = s.ChannelDelete(c.id)

				lit.Debug("deleted %s", c.name)

				continue
			}
		}

		lit.Debug("iteration %d, channel name %s", i, c.name)

		if len(server[v.GuildID].orderedChannels)-1 == i && server[v.GuildID].channels[c.id].connectedPeople > 0 {
			newChannel, err := s.GuildChannelCreate(v.GuildID, server[v.GuildID].prefix+" "+strconv.Itoa(i+2), discordgo.ChannelTypeGuildVoice)
			if err != nil {
				lit.Error(err.Error())
			}

			previousChannel, _ := s.Channel(c.id)

			if newChannel != nil {
				server[v.GuildID].channels[newChannel.ID] = &Channel{
					name:            newChannel.Name,
					id:              newChannel.ID,
					connectedPeople: 0,
				}

				server[v.GuildID].orderedChannels = append(server[v.GuildID].orderedChannels, *server[v.GuildID].channels[newChannel.ID])

				_, _ = s.ChannelEditComplex(newChannel.ID, &discordgo.ChannelEdit{ParentID: server[v.GuildID].category, Position: previousChannel.Position + 1})

			}
		}
	}

	server[v.GuildID].mutex.Unlock()

}
