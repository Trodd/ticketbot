package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ticketbot/config"
	"ticketbot/database"
	"ticketbot/handlers"

	"github.com/bwmarrin/discordgo"
)

func main() {
	log.Println("Starting Discord Ticket Bot...")

	// Load configuration
	cfg := config.Load()

	// Initialize database
	db, err := database.New("tickets.db")
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()
	log.Println("Database initialized")

	// Create Discord session
	session, err := discordgo.New("Bot " + cfg.Token)
	if err != nil {
		log.Fatalf("Failed to create Discord session: %v", err)
	}

	// Set HTTP client timeout (30 seconds)
	session.Client.Timeout = 30 * time.Second

	// Set intents
	session.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMessages

	// Initialize handlers
	h := handlers.New(cfg, db)

	// Register event handlers
	session.AddHandler(h.HandleInteraction)
	session.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		log.Printf("Logged in as %s#%s", s.State.User.Username, s.State.User.Discriminator)
		
		// Set bot status
		s.UpdateGameStatus(0, "Managing Tickets | /ticket")
	})

	// Open connection
	if err := session.Open(); err != nil {
		log.Fatalf("Failed to open Discord connection: %v", err)
	}
	defer session.Close()

	// Register slash commands asynchronously (Discord API can be slow)
	go func() {
		log.Println("Registering slash commands...")
		if err := h.RegisterCommands(session); err != nil {
			log.Printf("Warning: Failed to register commands: %v", err)
		} else {
			log.Println("Slash commands registered successfully")
		}
	}()

	// Wait for interrupt signal
	log.Println("Bot is now running. Press Ctrl+C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	log.Println("Shutting down...")
}
