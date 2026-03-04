package config

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// TicketCategory represents a configurable ticket category
type TicketCategory struct {
	ID           string   `json:"id"`           // Internal identifier (e.g., "support", "appeal")
	Name         string   `json:"name"`         // Display name (e.g., "Support Ticket")
	Description  string   `json:"description"`  // Button/command description
	Emoji        string   `json:"emoji"`        // Emoji for buttons
	AllowedRoles []string `json:"allowedRoles"` // Role IDs that can see this category (empty = support role only)
	Fields       []Field  `json:"fields"`       // Custom form fields
	HasApproval  bool     `json:"hasApproval"`  // Whether this category has approve/deny workflow
	Color        int      `json:"color"`        // Embed color (hex as int)
}

// Field represents a custom form field
type Field struct {
	ID          string `json:"id"`          // Field identifier
	Label       string `json:"label"`       // Display label
	Placeholder string `json:"placeholder"` // Placeholder text
	Required    bool   `json:"required"`    // Is field required
	Multiline   bool   `json:"multiline"`   // Use paragraph input
	MinLength   int    `json:"minLength"`   // Minimum length (0 = no min)
	MaxLength   int    `json:"maxLength"`   // Maximum length (0 = default 1000)
}

// Config holds all configuration for the bot
type Config struct {
	Token            string
	GuildID          string
	TicketCategoryID string // Discord category channel for tickets
	SupportRoleID    string
	LogChannelID     string
	Categories       []TicketCategory
}

// Load reads configuration from environment variables
func Load() *Config {
	// Load .env file if it exists
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	cfg := &Config{
		Token:            getEnv("DISCORD_TOKEN", ""),
		GuildID:          getEnv("GUILD_ID", ""),
		TicketCategoryID: getEnv("TICKET_CATEGORY_ID", ""),
		SupportRoleID:    getEnv("SUPPORT_ROLE_ID", ""),
		LogChannelID:     getEnv("LOG_CHANNEL_ID", ""),
	}

	if cfg.Token == "" {
		log.Fatal("DISCORD_TOKEN is required")
	}

	// Parse ticket categories from simple line-by-line format
	cfg.Categories = parseCategories()

	// If no categories configured, use defaults
	if len(cfg.Categories) == 0 {
		cfg.Categories = defaultCategories()
	}

	// Validate and set defaults for each category
	for i := range cfg.Categories {
		if cfg.Categories[i].MaxFieldLength() == 0 {
			// Set default max lengths for fields
			for j := range cfg.Categories[i].Fields {
				if cfg.Categories[i].Fields[j].MaxLength == 0 {
					cfg.Categories[i].Fields[j].MaxLength = 1000
				}
			}
		}
		if cfg.Categories[i].Color == 0 {
			cfg.Categories[i].Color = 0x5865F2 // Default Discord blurple
		}
	}

	return cfg
}

// parseCategories parses simple line-by-line category configuration
func parseCategories() []TicketCategory {
	var categories []TicketCategory

	// Support up to 10 categories
	for catNum := 1; catNum <= 10; catNum++ {
		prefix := "CATEGORY_" + strconv.Itoa(catNum) + "_"

		id := getEnv(prefix+"ID", "")
		if id == "" {
			continue // Skip if no ID defined
		}

		cat := TicketCategory{
			ID:          strings.ToLower(strings.TrimSpace(id)),
			Name:        getEnv(prefix+"NAME", id),
			Description: getEnv(prefix+"DESCRIPTION", ""),
			Emoji:       getEnv(prefix+"EMOJI", "🎫"),
			HasApproval: strings.EqualFold(getEnv(prefix+"HAS_APPROVAL", "false"), "true"),
			Color:       parseColor(getEnv(prefix+"COLOR", "5793266")),
		}

		// Parse allowed roles (comma-separated)
		rolesStr := getEnv(prefix+"ROLES", "")
		if rolesStr != "" {
			for _, role := range strings.Split(rolesStr, ",") {
				role = strings.TrimSpace(role)
				if role != "" {
					cat.AllowedRoles = append(cat.AllowedRoles, role)
				}
			}
		}

		// Parse up to 5 fields per category
		for fieldNum := 1; fieldNum <= 5; fieldNum++ {
			fieldPrefix := prefix + "FIELD_" + strconv.Itoa(fieldNum) + "_"

			fieldID := getEnv(fieldPrefix+"ID", "")
			if fieldID == "" {
				continue // Skip if no field ID
			}

			field := Field{
				ID:          strings.ToLower(strings.TrimSpace(fieldID)),
				Label:       getEnv(fieldPrefix+"LABEL", fieldID),
				Placeholder: getEnv(fieldPrefix+"PLACEHOLDER", ""),
				Required:    !strings.EqualFold(getEnv(fieldPrefix+"REQUIRED", "true"), "false"),
				Multiline:   strings.EqualFold(getEnv(fieldPrefix+"MULTILINE", "false"), "true"),
				MaxLength:   parseInt(getEnv(fieldPrefix+"MAX_LENGTH", "1000")),
			}
			cat.Fields = append(cat.Fields, field)
		}

		// Only add category if it has at least one field
		if len(cat.Fields) > 0 {
			categories = append(categories, cat)
		}
	}

	return categories
}

// parseColor converts a color string (decimal or hex) to int
func parseColor(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 5793266 // Default blurple
	}
	// Handle hex format
	if strings.HasPrefix(s, "#") {
		s = s[1:]
		n, _ := strconv.ParseInt(s, 16, 32)
		return int(n)
	}
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		s = s[2:]
		n, _ := strconv.ParseInt(s, 16, 32)
		return int(n)
	}
	// Assume decimal
	n, _ := strconv.Atoi(s)
	return n
}

func parseInt(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

// GetCategory returns a category by ID
func (c *Config) GetCategory(id string) *TicketCategory {
	for i := range c.Categories {
		if strings.EqualFold(c.Categories[i].ID, id) {
			return &c.Categories[i]
		}
	}
	return nil
}

// MaxFieldLength returns the max field length for the category (helper)
func (tc *TicketCategory) MaxFieldLength() int {
	max := 0
	for _, f := range tc.Fields {
		if f.MaxLength > max {
			max = f.MaxLength
		}
	}
	return max
}

func defaultCategories() []TicketCategory {
	return []TicketCategory{
		{
			ID:          "support",
			Name:        "Support Ticket",
			Description: "Get help from our support team",
			Emoji:       "🎫",
			HasApproval: false,
			Color:       0x5865F2,
			Fields: []Field{
				{ID: "subject", Label: "Subject", Placeholder: "Brief summary of your issue", Required: true, Multiline: false, MaxLength: 100},
				{ID: "description", Label: "Description", Placeholder: "Describe your issue in detail", Required: true, Multiline: true, MaxLength: 1000},
			},
		},
		{
			ID:          "appeal",
			Name:        "Ban Appeal",
			Description: "Appeal a ban or punishment",
			Emoji:       "⚖️",
			HasApproval: true,
			Color:       0xFEE75C,
			Fields: []Field{
				{ID: "punishment", Label: "What punishment are you appealing?", Placeholder: "e.g., Ban, Mute, Warn", Required: true, Multiline: false, MaxLength: 100},
				{ID: "reason", Label: "Why should we accept your appeal?", Placeholder: "Explain why you should be unbanned", Required: true, Multiline: true, MaxLength: 1000},
			},
		},
		{
			ID:          "application",
			Name:        "Staff Application",
			Description: "Apply for a staff position",
			Emoji:       "📝",
			HasApproval: true,
			Color:       0x57F287,
			Fields: []Field{
				{ID: "position", Label: "Position", Placeholder: "What position are you applying for?", Required: true, Multiline: false, MaxLength: 100},
				{ID: "experience", Label: "Experience", Placeholder: "Describe your relevant experience", Required: true, Multiline: true, MaxLength: 1000},
				{ID: "why", Label: "Why do you want to join?", Placeholder: "Tell us why you'd be a good fit", Required: true, Multiline: true, MaxLength: 1000},
			},
		},
		{
			ID:          "report",
			Name:        "Report",
			Description: "Report a user or issue",
			Emoji:       "🚨",
			HasApproval: false,
			Color:       0xED4245,
			Fields: []Field{
				{ID: "reported", Label: "Who/What are you reporting?", Placeholder: "Username or description", Required: true, Multiline: false, MaxLength: 200},
				{ID: "details", Label: "Details", Placeholder: "Provide details about the incident", Required: true, Multiline: true, MaxLength: 1000},
				{ID: "evidence", Label: "Evidence", Placeholder: "Links to screenshots, etc. (optional)", Required: false, Multiline: true, MaxLength: 500},
			},
		},
	}
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
