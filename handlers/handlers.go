package handlers

import (
	"bytes"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"ticketbot/config"
	"ticketbot/database"

	"github.com/bwmarrin/discordgo"
)

var (
	urlRegex        = regexp.MustCompile(`(https?://[^\s<>"]+)`)
	youtubeRegex    = regexp.MustCompile(`(?:youtube\.com/watch\?v=|youtu\.be/|youtube\.com/shorts/)([a-zA-Z0-9_-]+)`)
	twitchClipRegex = regexp.MustCompile(`clips\.twitch\.tv/([a-zA-Z0-9_-]+)`)
	imageRegex      = regexp.MustCompile(`(?i)\.(jpg|jpeg|png|gif|webp)(\?.*)?$`)
)

// Handler manages Discord event handlers
type Handler struct {
	cfg *config.Config
	db  *database.DB
}

// New creates a new Handler
func New(cfg *config.Config, db *database.DB) *Handler {
	return &Handler{
		cfg: cfg,
		db:  db,
	}
}

// RegisterCommands registers slash commands with Discord
func (h *Handler) RegisterCommands(s *discordgo.Session) error {
	commands := []*discordgo.ApplicationCommand{
		{
			Name:        "close",
			Description: "Close the current ticket",
		},
		{
			Name:        "approve",
			Description: "Approve this ticket (Staff only)",
		},
		{
			Name:        "deny",
			Description: "Deny this ticket (Staff only)",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "reason",
					Description: "Reason for denial",
					Required:    false,
				},
			},
		},
		{
			Name:        "priority",
			Description: "Set ticket priority (Staff only)",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "level",
					Description: "Priority level",
					Required:    true,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{Name: "🔴 Urgent", Value: "urgent"},
						{Name: "🟠 High", Value: "high"},
						{Name: "🟢 Normal", Value: "normal"},
						{Name: "⚪ Low", Value: "low"},
					},
				},
			},
		},
		{
			Name:        "note",
			Description: "Add an internal staff note (Staff only)",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "content",
					Description: "Note content",
					Required:    true,
				},
			},
		},
		{
			Name:        "notes",
			Description: "View staff notes for this ticket (Staff only)",
		},
		{
			Name:        "history",
			Description: "View a user's ticket history (Staff only)",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionUser,
					Name:        "user",
					Description: "User to check",
					Required:    true,
				},
			},
		},
		{
			Name:        "blacklist",
			Description: "Manage ticket blacklist (Staff only)",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "add",
					Description: "Add user to blacklist",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionUser,
							Name:        "user",
							Description: "User to blacklist",
							Required:    true,
						},
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "reason",
							Description: "Reason for blacklist",
							Required:    true,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "remove",
					Description: "Remove user from blacklist",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionUser,
							Name:        "user",
							Description: "User to remove",
							Required:    true,
						},
					},
				},
			},
		},
		{
			Name:        "transcript",
			Description: "Save ticket transcript (Staff only)",
		},
		{
			Name:        "ticketpanel",
			Description: "Create a ticket panel with buttons (Admin only)",
		},
		{
			Name:        "ticketstats",
			Description: "View ticket statistics",
		},
		{
			Name:        "adduser",
			Description: "Add a user to the current ticket",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionUser,
					Name:        "user",
					Description: "User to add",
					Required:    true,
				},
			},
		},
		{
			Name:        "removeuser",
			Description: "Remove a user from the current ticket",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionUser,
					Name:        "user",
					Description: "User to remove",
					Required:    true,
				},
			},
		},
		{
			Name:        "queue",
			Description: "View open tickets (Staff only)",
		},
	}

	// Dynamically add commands for each configured category
	for _, cat := range h.cfg.Categories {
		commands = append(commands, &discordgo.ApplicationCommand{
			Name:        cat.ID,
			Description: cat.Description,
		})
	}

	guildID := h.cfg.GuildID

	// Register commands one by one (more reliable than bulk)
	log.Printf("Registering %d commands for guild %s...", len(commands), guildID)
	for _, cmd := range commands {
		_, err := s.ApplicationCommandCreate(s.State.User.ID, guildID, cmd)
		if err != nil {
			log.Printf("Warning: Failed to register command %s: %v", cmd.Name, err)
		} else {
			log.Printf("Registered: /%s", cmd.Name)
		}
	}

	log.Printf("Command registration complete")
	return nil
}

// HandleInteraction routes interactions to appropriate handlers
func (h *Handler) HandleInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	switch i.Type {
	case discordgo.InteractionApplicationCommand:
		h.handleCommand(s, i)
	case discordgo.InteractionMessageComponent:
		h.handleComponent(s, i)
	case discordgo.InteractionModalSubmit:
		h.handleModalSubmit(s, i)
	}
}

func (h *Handler) handleCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	cmdName := i.ApplicationCommandData().Name

	// Check if it's a dynamic category command
	if cat := h.cfg.GetCategory(cmdName); cat != nil {
		h.showTicketModal(s, i, cat.ID)
		return
	}

	// Handle built-in commands
	switch cmdName {
	case "close":
		h.handleTicketClose(s, i)
	case "approve":
		h.handleApprove(s, i)
	case "deny":
		h.handleDeny(s, i)
	case "priority":
		h.handlePriority(s, i)
	case "note":
		h.handleAddNote(s, i)
	case "notes":
		h.handleViewNotes(s, i)
	case "history":
		h.handleHistory(s, i)
	case "blacklist":
		h.handleBlacklist(s, i)
	case "transcript":
		h.handleTranscript(s, i)
	case "ticketpanel":
		h.handleTicketPanel(s, i)
	case "ticketstats":
		h.handleTicketStats(s, i)
	case "adduser":
		h.handleAddUser(s, i)
	case "removeuser":
		h.handleRemoveUser(s, i)
	case "queue":
		h.handleQueue(s, i)
	}
}

func (h *Handler) handleComponent(s *discordgo.Session, i *discordgo.InteractionCreate) {
	customID := i.MessageComponentData().CustomID

	// Check for dynamic category button (create_ticket_<categoryID>)
	if strings.HasPrefix(customID, "create_ticket_") {
		categoryID := strings.TrimPrefix(customID, "create_ticket_")
		if cat := h.cfg.GetCategory(categoryID); cat != nil {
			h.showTicketModal(s, i, cat.ID)
			return
		}
	}

	switch {
	case customID == "close_ticket":
		h.handleTicketClose(s, i)
	case strings.HasPrefix(customID, "confirm_close_"):
		h.handleConfirmClose(s, i)
	case customID == "cancel_close":
		h.handleCancelClose(s, i)
	case customID == "reopen_ticket":
		h.handleReopenTicket(s, i)
	case customID == "delete_ticket":
		h.handleDeleteTicket(s, i)
	case strings.HasPrefix(customID, "approve_"):
		h.handleApprove(s, i)
	case strings.HasPrefix(customID, "deny_"):
		h.showDenyModal(s, i)
	}
}

func (h *Handler) handleModalSubmit(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ModalSubmitData()

	if strings.HasPrefix(data.CustomID, "ticket_modal_") {
		categoryID := strings.TrimPrefix(data.CustomID, "ticket_modal_")
		h.processTicketModal(s, i, categoryID)
	} else if strings.HasPrefix(data.CustomID, "deny_reason_modal_") {
		h.processDenyModal(s, i)
	}
}

func (h *Handler) showDenyModal(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !h.isStaff(i) {
		h.respond(s, i, "❌ Only staff members can deny tickets.", true)
		return
	}

	// Include message ID in modal custom ID so we can delete original message later
	messageID := ""
	if i.Message != nil {
		messageID = i.Message.ID
	}

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			CustomID: "deny_reason_modal_" + messageID,
			Title:    "Deny Ticket",
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    "reason",
							Label:       "Reason for Denial",
							Style:       discordgo.TextInputParagraph,
							Placeholder: "Explain why this appeal/application is being denied...",
							Required:    true,
							MinLength:   5,
							MaxLength:   1000,
						},
					},
				},
			},
		},
	})
	if err != nil {
		log.Printf("Failed to show deny modal: %v", err)
	}
}

func (h *Handler) processDenyModal(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ModalSubmitData()
	reason := ""
	for _, comp := range data.Components {
		if row, ok := comp.(*discordgo.ActionsRow); ok {
			for _, rowComp := range row.Components {
				if input, ok := rowComp.(*discordgo.TextInput); ok && input.CustomID == "reason" {
					reason = input.Value
				}
			}
		}
	}

	ticket, err := h.db.GetTicketByChannel(i.ChannelID)
	if err != nil || ticket == nil {
		h.respond(s, i, "This can only be used in a ticket channel.", true)
		return
	}

	staffID := h.getUserID(i)
	if err := h.db.DenyTicket(i.ChannelID, staffID); err != nil {
		h.respondError(s, i, "Failed to deny ticket")
		return
	}

	// Send denial notification as a new message (keep original ticket embed)
	denyColor := 0xED4245
	container := discordgo.Container{
		AccentColor: &denyColor,
		Components: []discordgo.MessageComponent{
			discordgo.TextDisplay{Content: fmt.Sprintf("# ❌ Ticket Denied\n\n**Staff:** <@%s>\n**Reason:** %s", staffID, reason)},
			discordgo.Separator{},
			discordgo.ActionsRow{Components: []discordgo.MessageComponent{
				discordgo.Button{Label: "Close", Style: discordgo.SecondaryButton, CustomID: "close_ticket", Emoji: &discordgo.ComponentEmoji{Name: "🔒"}},
			}},
		},
	}

	s.ChannelMessageSendComplex(i.ChannelID, &discordgo.MessageSend{
		Content:    fmt.Sprintf("<@%s>", ticket.UserID),
		Flags:      discordgo.MessageFlagsIsComponentsV2,
		Components: []discordgo.MessageComponent{container},
	})

	// Acknowledge the modal silently
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
	})
}

func (h *Handler) showTicketModal(s *discordgo.Session, i *discordgo.InteractionCreate, categoryID string) {
	userID := h.getUserID(i)

	// Get category config
	cat := h.cfg.GetCategory(categoryID)
	if cat == nil {
		h.respond(s, i, "❌ Unknown ticket category.", true)
		return
	}

	// Check blacklist
	bl, err := h.db.IsBlacklisted(userID, i.GuildID)
	if err == nil && bl != nil {
		h.respond(s, i, fmt.Sprintf("❌ You are blacklisted from creating tickets.\n**Reason:** %s", bl.Reason), true)
		return
	}

	// Check existing tickets
	openTickets, err := h.db.GetOpenTicketsByUser(userID, i.GuildID)
	if err != nil {
		h.respondError(s, i, "Failed to check existing tickets")
		return
	}
	if len(openTickets) >= 3 {
		h.respond(s, i, "You already have 3 open tickets. Please close one before creating a new one.", true)
		return
	}

	// Build modal title
	title := fmt.Sprintf("%s %s", cat.Emoji, cat.Name)
	if len(title) > 45 {
		title = title[:45]
	}

	// Build modal components from category fields (max 5 fields for Discord modals)
	var components []discordgo.MessageComponent
	for idx, field := range cat.Fields {
		if idx >= 5 {
			break // Discord modals support max 5 text inputs
		}

		style := discordgo.TextInputShort
		if field.Multiline {
			style = discordgo.TextInputParagraph
		}

		maxLen := field.MaxLength
		if maxLen == 0 {
			maxLen = 1000
		}

		components = append(components, discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.TextInput{
				CustomID:    field.ID,
				Label:       field.Label,
				Style:       style,
				Placeholder: field.Placeholder,
				Required:    field.Required,
				MinLength:   field.MinLength,
				MaxLength:   maxLen,
			},
		}})
	}

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			CustomID:   "ticket_modal_" + categoryID,
			Title:      title,
			Components: components,
		},
	})
	if err != nil {
		log.Printf("Failed to show modal: %v", err)
	}
}

func (h *Handler) processTicketModal(s *discordgo.Session, i *discordgo.InteractionCreate, categoryID string) {
	userID := h.getUserID(i)

	// Get category config
	cat := h.cfg.GetCategory(categoryID)
	if cat == nil {
		h.respond(s, i, "❌ Unknown ticket category.", true)
		return
	}

	// Extract form data
	formData := make(map[string]string)
	for _, comp := range i.ModalSubmitData().Components {
		if row, ok := comp.(*discordgo.ActionsRow); ok {
			for _, rowComp := range row.Components {
				if input, ok := rowComp.(*discordgo.TextInput); ok {
					formData[input.CustomID] = input.Value
				}
			}
		}
	}

	// Determine subject - use first field value or category name
	var subject string
	if len(cat.Fields) > 0 {
		firstFieldID := cat.Fields[0].ID
		subject = fmt.Sprintf("%s: %s", cat.Name, formData[firstFieldID])
	} else {
		subject = cat.Name
	}

	// Defer response
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Flags: discordgo.MessageFlagsEphemeral},
	})

	// Get next ticket number for this type
	nextNum := h.db.GetNextTicketNumberByType(i.GuildID, database.TicketType(categoryID))

	// Create channel with type-specific naming
	channelName := fmt.Sprintf("%s-%d", categoryID, nextNum)

	// Build permission overwrites
	overwrites := []*discordgo.PermissionOverwrite{
		{ID: i.GuildID, Type: discordgo.PermissionOverwriteTypeRole, Deny: discordgo.PermissionViewChannel},
		{ID: userID, Type: discordgo.PermissionOverwriteTypeMember, Allow: discordgo.PermissionViewChannel | discordgo.PermissionSendMessages | discordgo.PermissionReadMessageHistory},
	}

	// Add support role
	if h.cfg.SupportRoleID != "" {
		overwrites = append(overwrites, &discordgo.PermissionOverwrite{
			ID: h.cfg.SupportRoleID, Type: discordgo.PermissionOverwriteTypeRole,
			Allow: discordgo.PermissionViewChannel | discordgo.PermissionSendMessages | discordgo.PermissionReadMessageHistory | discordgo.PermissionManageMessages,
		})
	}

	// Add category-specific allowed roles
	for _, roleID := range cat.AllowedRoles {
		if roleID != "" && roleID != h.cfg.SupportRoleID {
			overwrites = append(overwrites, &discordgo.PermissionOverwrite{
				ID: roleID, Type: discordgo.PermissionOverwriteTypeRole,
				Allow: discordgo.PermissionViewChannel | discordgo.PermissionSendMessages | discordgo.PermissionReadMessageHistory,
			})
		}
	}

	channelData := discordgo.GuildChannelCreateData{
		Name:                 channelName,
		Type:                 discordgo.ChannelTypeGuildText,
		PermissionOverwrites: overwrites,
	}
	if h.cfg.TicketCategoryID != "" {
		channelData.ParentID = h.cfg.TicketCategoryID
	}

	channel, err := s.GuildChannelCreateComplex(i.GuildID, channelData)
	if err != nil {
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{Content: stringPtr("Failed to create ticket channel.")})
		log.Printf("Failed to create channel: %v", err)
		return
	}

	// Get description for database - use second field or first field
	var description string
	if len(cat.Fields) > 1 {
		description = formData[cat.Fields[1].ID]
	} else if len(cat.Fields) > 0 {
		description = formData[cat.Fields[0].ID]
	}

	// Save ticket
	ticket, err := h.db.CreateTicket(channel.ID, userID, i.GuildID, database.TicketType(categoryID), subject, description, formData)
	if err != nil {
		s.ChannelDelete(channel.ID)
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{Content: stringPtr("Failed to save ticket.")})
		log.Printf("Failed to save ticket: %v", err)
		return
	}

	// Build action buttons
	var buttons []discordgo.MessageComponent

	if cat.HasApproval {
		buttons = append(buttons, discordgo.Button{Label: "Approve", Style: discordgo.SuccessButton, CustomID: "approve_" + channel.ID, Emoji: &discordgo.ComponentEmoji{Name: "✅"}})
		buttons = append(buttons, discordgo.Button{Label: "Deny", Style: discordgo.DangerButton, CustomID: "deny_" + channel.ID, Emoji: &discordgo.ComponentEmoji{Name: "❌"}})
	}
	buttons = append(buttons, discordgo.Button{Label: "Close", Style: discordgo.SecondaryButton, CustomID: "close_ticket", Emoji: &discordgo.ComponentEmoji{Name: "🔒"}})

	// Build Components v2 message
	components := h.buildTicketComponentsV2(ticket, cat, userID, formData, buttons, "")

	// Send the Components v2 ticket message (mentions user in the component)
	s.ChannelMessageSendComplex(channel.ID, &discordgo.MessageSend{
		Flags:      discordgo.MessageFlagsIsComponentsV2,
		Components: components,
	})

	// Send media links using Components v2 - MediaGallery for images, video links separately
	// Look for any field that might contain links
	var links string
	for _, val := range formData {
		foundURLs := urlRegex.FindAllString(val, -1)
		if len(foundURLs) > 0 {
			links += " " + val
		}
	}

	if links != "" {
		allURLs := urlRegex.FindAllString(links, -1)
		var imageItems []discordgo.MediaGalleryItem
		var videos []string

		for _, url := range allURLs {
			if imageRegex.MatchString(url) {
				// Image - add to gallery
				imageItems = append(imageItems, discordgo.MediaGalleryItem{
					Media: discordgo.UnfurledMediaItem{URL: url},
				})
			} else if youtubeRegex.MatchString(url) || twitchClipRegex.MatchString(url) {
				// Video - send separately for embedding
				videos = append(videos, url)
			}
		}

		// Send images as MediaGallery (up to 10)
		if len(imageItems) > 0 {
			if len(imageItems) > 10 {
				imageItems = imageItems[:10]
			}
			s.ChannelMessageSendComplex(channel.ID, &discordgo.MessageSend{
				Flags: discordgo.MessageFlagsIsComponentsV2,
				Components: []discordgo.MessageComponent{
					discordgo.MediaGallery{Items: imageItems},
				},
			})
		}

		// Send video links separately for embedding
		for _, url := range videos {
			s.ChannelMessageSend(channel.ID, fmt.Sprintf("📹 **Video Link:**\n%s", url))
		}
	}

	// Transcript will be auto-logged when ticket is closed

	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: stringPtr(fmt.Sprintf("✅ Your %s ticket has been created: <#%s>", cat.Name, channel.ID)),
	})
}

func (h *Handler) buildTicketEmbed(ticket *database.Ticket, cat *config.TicketCategory, userID string, formData map[string]string) *discordgo.MessageEmbed {
	var fields []*discordgo.MessageEmbedField

	// Add standard fields
	fields = append(fields, &discordgo.MessageEmbedField{Name: "Ticket ID", Value: fmt.Sprintf("#%d", ticket.TicketNumber), Inline: true})
	fields = append(fields, &discordgo.MessageEmbedField{Name: "User", Value: fmt.Sprintf("<@%s>", userID), Inline: true})
	fields = append(fields, &discordgo.MessageEmbedField{Name: "Status", Value: "🟡 Pending", Inline: true})

	// Add dynamic fields from form data
	for _, field := range cat.Fields {
		if val, ok := formData[field.ID]; ok && val != "" {
			fields = append(fields, &discordgo.MessageEmbedField{
				Name:   field.Label,
				Value:  truncate(val, 1000),
				Inline: false,
			})
		}
	}

	return &discordgo.MessageEmbed{
		Title:  fmt.Sprintf("%s %s", cat.Emoji, cat.Name),
		Color:  cat.Color,
		Fields: fields,
		Footer: &discordgo.MessageEmbedFooter{Text: "Staff: Use the buttons to manage this ticket"},
	}
}

// buildTicketComponentsV2 creates Components v2 layout for ticket messages
// statusOverride can be empty for default, or contain status text like "✅ APPROVED" or "❌ DENIED"
func (h *Handler) buildTicketComponentsV2(ticket *database.Ticket, cat *config.TicketCategory, userID string, formData map[string]string, buttons []discordgo.MessageComponent, statusOverride string) []discordgo.MessageComponent {
	dividerTrue := true
	spacingSmall := discordgo.SeparatorSpacingSizeSmall

	// Determine status display and color
	statusText := "🟡 Pending"
	color := cat.Color
	if statusOverride != "" {
		statusText = statusOverride
		if strings.Contains(statusOverride, "APPROVED") {
			color = 0x57F287
		} else if strings.Contains(statusOverride, "DENIED") {
			color = 0xED4245
		} else if strings.Contains(statusOverride, "Closed") {
			color = 0x95A5A6
		}
	}

	// Build content parts
	var contentParts []discordgo.MessageComponent

	// Header
	contentParts = append(contentParts, discordgo.TextDisplay{Content: fmt.Sprintf("# %s %s", cat.Emoji, cat.Name)})
	contentParts = append(contentParts, discordgo.TextDisplay{Content: fmt.Sprintf("**Ticket ID:** #%d  •  **User:** <@%s>  •  **Created:** <t:%d:R>  •  **Status:** %s", ticket.TicketNumber, userID, ticket.CreatedAt.Unix(), statusText)})
	contentParts = append(contentParts, discordgo.Separator{Divider: &dividerTrue, Spacing: &spacingSmall})

	// Dynamic fields from form data
	for _, field := range cat.Fields {
		if val, ok := formData[field.ID]; ok && val != "" {
			contentParts = append(contentParts, discordgo.TextDisplay{Content: fmt.Sprintf("**%s:**\n%s", field.Label, truncate(val, 1000))})
		}
	}

	// Add footer and action buttons
	contentParts = append(contentParts, discordgo.Separator{Divider: &dividerTrue, Spacing: &spacingSmall})
	contentParts = append(contentParts, discordgo.TextDisplay{Content: "*Staff: Use the buttons below to manage this ticket*"})
	contentParts = append(contentParts, discordgo.ActionsRow{Components: buttons})

	return []discordgo.MessageComponent{
		discordgo.Container{
			AccentColor: &color,
			Components:  contentParts,
		},
	}
}

func (h *Handler) handleApprove(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !h.isStaff(i) {
		h.respond(s, i, "❌ Only staff members can approve tickets.", true)
		return
	}

	ticket, err := h.db.GetTicketByChannel(i.ChannelID)
	if err != nil || ticket == nil {
		h.respond(s, i, "This command can only be used in a ticket channel.", true)
		return
	}

	// Check if category supports approval workflow
	cat := h.cfg.GetCategory(string(ticket.Type))
	if cat == nil || !cat.HasApproval {
		h.respond(s, i, "This ticket type does not support approval workflow.", true)
		return
	}

	staffID := h.getUserID(i)
	if err := h.db.ApproveTicket(i.ChannelID, staffID); err != nil {
		h.respondError(s, i, "Failed to approve ticket")
		return
	}

	// Send approval notification as a new message (keep original ticket embed)
	approveColor := 0x57F287
	container := discordgo.Container{
		AccentColor: &approveColor,
		Components: []discordgo.MessageComponent{
			discordgo.TextDisplay{Content: fmt.Sprintf("# ✅ Ticket Approved\n\n**Staff:** <@%s>", staffID)},
			discordgo.Separator{},
			discordgo.ActionsRow{Components: []discordgo.MessageComponent{
				discordgo.Button{Label: "Close", Style: discordgo.SecondaryButton, CustomID: "close_ticket", Emoji: &discordgo.ComponentEmoji{Name: "🔒"}},
			}},
		},
	}

	// Send new message with ping to notify user
	s.ChannelMessageSendComplex(i.ChannelID, &discordgo.MessageSend{
		Content:    fmt.Sprintf("<@%s>", ticket.UserID),
		Flags:      discordgo.MessageFlagsIsComponentsV2,
		Components: []discordgo.MessageComponent{container},
	})

	// Acknowledge the button click
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
	})
}

func (h *Handler) handleDeny(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !h.isStaff(i) {
		h.respond(s, i, "❌ Only staff members can deny tickets.", true)
		return
	}

	ticket, err := h.db.GetTicketByChannel(i.ChannelID)
	if err != nil || ticket == nil {
		h.respond(s, i, "This command can only be used in a ticket channel.", true)
		return
	}

	// Get denial reason
	reason := "No reason provided"
	if i.Type == discordgo.InteractionApplicationCommand {
		options := i.ApplicationCommandData().Options
		for _, opt := range options {
			if opt.Name == "reason" {
				reason = opt.StringValue()
			}
		}
	}

	staffID := h.getUserID(i)
	if err := h.db.DenyTicket(i.ChannelID, staffID); err != nil {
		h.respondError(s, i, "Failed to deny ticket")
		return
	}

	// Use Components v2 to notify user
	denyColor := 0xED4245
	spacingSmall := discordgo.SeparatorSpacingSizeSmall
	divider := true
	container := discordgo.Container{
		AccentColor: &denyColor,
		Components: []discordgo.MessageComponent{
			discordgo.TextDisplay{Content: fmt.Sprintf("# ❌ DENIED\n\nThis %s has been **DENIED** by <@%s>.", ticket.Type, staffID)},
			discordgo.Separator{Divider: &divider, Spacing: &spacingSmall},
			discordgo.TextDisplay{Content: fmt.Sprintf("**Reason:** %s", reason)},
			discordgo.Separator{Divider: &divider, Spacing: &spacingSmall},
			discordgo.TextDisplay{Content: fmt.Sprintf("<@%s> Your %s has been denied. Please review the reason above.", ticket.UserID, ticket.Type)},
			discordgo.Separator{Divider: &divider, Spacing: &spacingSmall},
			discordgo.TextDisplay{Content: "*Use the Close button to close this ticket when ready.*"},
		},
	}

	s.ChannelMessageSendComplex(i.ChannelID, &discordgo.MessageSend{
		Flags:      discordgo.MessageFlagsIsComponentsV2,
		Components: []discordgo.MessageComponent{container},
	})

	h.respond(s, i, "✅ Ticket denied. User has been notified.", true)
}

func (h *Handler) handlePriority(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !h.isStaff(i) {
		h.respond(s, i, "❌ Only staff members can set priority.", true)
		return
	}

	ticket, err := h.db.GetTicketByChannel(i.ChannelID)
	if err != nil || ticket == nil {
		h.respond(s, i, "This command can only be used in a ticket channel.", true)
		return
	}

	priority := i.ApplicationCommandData().Options[0].StringValue()
	if err := h.db.SetTicketPriority(i.ChannelID, priority); err != nil {
		h.respondError(s, i, "Failed to set priority")
		return
	}

	priorityEmoji := map[string]string{"urgent": "🔴", "high": "🟠", "normal": "🟢", "low": "⚪"}

	// Update channel name with priority dot
	newChannelName := fmt.Sprintf("%s-%s-%d", priorityEmoji[priority], ticket.Type, ticket.TicketNumber)
	_, err = s.ChannelEdit(i.ChannelID, &discordgo.ChannelEdit{Name: newChannelName})
	if err != nil {
		log.Printf("Failed to rename channel: %v", err)
	}

	h.respond(s, i, fmt.Sprintf("%s Ticket priority set to **%s**", priorityEmoji[priority], priority), false)
}

func (h *Handler) handleAddNote(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !h.isStaff(i) {
		h.respond(s, i, "❌ Only staff members can add notes.", true)
		return
	}

	ticket, err := h.db.GetTicketByChannel(i.ChannelID)
	if err != nil || ticket == nil {
		h.respond(s, i, "This command can only be used in a ticket channel.", true)
		return
	}

	content := i.ApplicationCommandData().Options[0].StringValue()
	staffID := h.getUserID(i)

	if _, err := h.db.AddStaffNote(ticket.ID, staffID, content); err != nil {
		h.respondError(s, i, "Failed to add note")
		return
	}

	h.respond(s, i, "📝 Staff note added.", true)
}

func (h *Handler) handleViewNotes(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !h.isStaff(i) {
		h.respond(s, i, "❌ Only staff members can view notes.", true)
		return
	}

	ticket, err := h.db.GetTicketByChannel(i.ChannelID)
	if err != nil || ticket == nil {
		h.respond(s, i, "This command can only be used in a ticket channel.", true)
		return
	}

	notes, err := h.db.GetStaffNotes(ticket.ID)
	if err != nil {
		h.respondError(s, i, "Failed to get notes")
		return
	}

	if len(notes) == 0 {
		h.respond(s, i, "No staff notes for this ticket.", true)
		return
	}

	var fields []*discordgo.MessageEmbedField
	for _, note := range notes {
		// Get the author's display name
		authorName := note.AuthorID
		if member, err := s.GuildMember(i.GuildID, note.AuthorID); err == nil && member.User != nil {
			if member.Nick != "" {
				authorName = member.Nick
			} else {
				authorName = member.User.Username
			}
		}

		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   fmt.Sprintf("%s - <t:%d:R>", authorName, note.CreatedAt.Unix()),
			Value:  note.Content,
			Inline: false,
		})
	}

	embed := &discordgo.MessageEmbed{
		Title:  "📝 Staff Notes",
		Color:  0x5865F2,
		Fields: fields,
	}
	h.respondEmbed(s, i, embed, true)
}

func (h *Handler) handleHistory(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !h.isStaff(i) {
		h.respond(s, i, "❌ Only staff members can view history.", true)
		return
	}

	targetUser := i.ApplicationCommandData().Options[0].UserValue(s)
	tickets, err := h.db.GetUserTicketHistory(targetUser.ID, i.GuildID, 10)
	if err != nil {
		h.respondError(s, i, "Failed to get history")
		return
	}

	if len(tickets) == 0 {
		h.respond(s, i, fmt.Sprintf("<@%s> has no ticket history.", targetUser.ID), true)
		return
	}

	var description strings.Builder
	for _, t := range tickets {
		status := string(t.Status)
		if t.Resolution != nil && *t.Resolution != "" {
			status = *t.Resolution
		}
		description.WriteString(fmt.Sprintf("**#%d** `%s` - %s - %s\n", t.TicketNumber, t.Type, t.Subject, status))
	}

	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("📜 Ticket History for %s", targetUser.Username),
		Description: description.String(),
		Color:       0x5865F2,
	}
	h.respondEmbed(s, i, embed, true)
}

func (h *Handler) handleBlacklist(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !h.isStaff(i) {
		h.respond(s, i, "❌ Only staff members can manage blacklist.", true)
		return
	}

	options := i.ApplicationCommandData().Options
	subCmd := options[0]
	staffID := h.getUserID(i)

	switch subCmd.Name {
	case "add":
		targetUser := subCmd.Options[0].UserValue(s)
		reason := subCmd.Options[1].StringValue()

		if err := h.db.AddToBlacklist(targetUser.ID, i.GuildID, reason, staffID, nil); err != nil {
			h.respondError(s, i, "Failed to add to blacklist")
			return
		}
		h.respond(s, i, fmt.Sprintf("✅ <@%s> has been blacklisted.\n**Reason:** %s", targetUser.ID, reason), false)

	case "remove":
		targetUser := subCmd.Options[0].UserValue(s)

		if err := h.db.RemoveFromBlacklist(targetUser.ID, i.GuildID); err != nil {
			h.respondError(s, i, "Failed to remove from blacklist")
			return
		}
		h.respond(s, i, fmt.Sprintf("✅ <@%s> has been removed from the blacklist.", targetUser.ID), false)
	}
}

func (h *Handler) handleTranscript(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !h.isStaff(i) {
		h.respond(s, i, "❌ Only staff members can save transcripts.", true)
		return
	}

	ticket, err := h.db.GetTicketByChannel(i.ChannelID)
	if err != nil || ticket == nil {
		h.respond(s, i, "This command can only be used in a ticket channel.", true)
		return
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Flags: discordgo.MessageFlagsEphemeral},
	})

	// Fetch messages
	messages, err := s.ChannelMessages(i.ChannelID, 100, "", "", "")
	if err != nil {
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{Content: stringPtr("Failed to fetch messages.")})
		return
	}

	// Build transcript
	var transcript strings.Builder
	transcript.WriteString(fmt.Sprintf("=== TICKET #%d TRANSCRIPT ===\n", ticket.TicketNumber))
	transcript.WriteString(fmt.Sprintf("Type: %s\n", ticket.Type))
	transcript.WriteString(fmt.Sprintf("User: %s\n", ticket.UserID))
	transcript.WriteString(fmt.Sprintf("Subject: %s\n", ticket.Subject))
	transcript.WriteString(fmt.Sprintf("Created: %s\n", ticket.CreatedAt.Format(time.RFC1123)))
	transcript.WriteString("=============================\n\n")

	// Reverse to chronological order
	for j := len(messages) - 1; j >= 0; j-- {
		msg := messages[j]
		timestamp := msg.Timestamp.Format("2006-01-02 15:04:05")
		transcript.WriteString(fmt.Sprintf("[%s] %s: %s\n", timestamp, msg.Author.Username, msg.Content))
	}

	// Save to database
	_, err = h.db.SaveTranscript(ticket.ID, i.ChannelID, i.GuildID, ticket.UserID, transcript.String())
	if err != nil {
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{Content: stringPtr("Failed to save transcript.")})
		return
	}

	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{Content: stringPtr("✅ Transcript saved!")})
}

func (h *Handler) handleQueue(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !h.isStaff(i) {
		h.respond(s, i, "❌ Only staff members can view the queue.", true)
		return
	}

	tickets, err := h.db.GetOpenTickets(i.GuildID)
	if err != nil {
		h.respondError(s, i, "Failed to get queue")
		return
	}

	if len(tickets) == 0 {
		h.respond(s, i, "✅ No open tickets!", true)
		return
	}

	var description strings.Builder
	priorityEmoji := map[string]string{"urgent": "🔴", "high": "🟠", "normal": "🟢", "low": "⚪"}
	for _, t := range tickets {
		emoji := priorityEmoji[t.Priority]
		description.WriteString(fmt.Sprintf("%s **#%d** `%s` <#%s>\n", emoji, t.TicketNumber, t.Type, t.ChannelID))
	}

	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("📥 Ticket Queue (%d)", len(tickets)),
		Description: description.String(),
		Color:       0x5865F2,
	}
	h.respondEmbed(s, i, embed, true)
}

func (h *Handler) handleTicketClose(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ticket, err := h.db.GetTicketByChannel(i.ChannelID)
	if err != nil || ticket == nil {
		h.respond(s, i, "This command can only be used in a ticket channel.", true)
		return
	}

	// Allow staff or ticket creator to close
	userID := h.getUserID(i)
	if !h.isStaff(i) && userID != ticket.UserID {
		h.respond(s, i, "❌ Only staff or the ticket creator can close this ticket.", true)
		return
	}

	if ticket.Status == database.StatusClosed {
		h.respond(s, i, "This ticket is already closed.", true)
		return
	}

	// Get category for this ticket
	cat := h.cfg.GetCategory(string(ticket.Type))
	if cat == nil {
		cat = &config.TicketCategory{ID: string(ticket.Type), Name: string(ticket.Type), Emoji: "🎫", Color: 0x5865F2}
	}

	// Build confirmation buttons
	buttons := []discordgo.MessageComponent{
		discordgo.Button{Label: "Confirm Close", Style: discordgo.DangerButton, CustomID: fmt.Sprintf("confirm_close_%s", i.ChannelID), Emoji: &discordgo.ComponentEmoji{Name: "✅"}},
		discordgo.Button{Label: "Cancel", Style: discordgo.SecondaryButton, CustomID: "cancel_close", Emoji: &discordgo.ComponentEmoji{Name: "❌"}},
	}

	// Update the message with confirmation prompt while preserving ticket info
	statusText := "⚠️ Confirm close? Non-staff will be removed"
	components := h.buildTicketComponentsV2(ticket, cat, ticket.UserID, ticket.FormData, buttons, statusText)

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Flags:      discordgo.MessageFlagsIsComponentsV2,
			Components: components,
		},
	})
}

func (h *Handler) handleConfirmClose(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ticket, err := h.db.GetTicketByChannel(i.ChannelID)
	if err != nil || ticket == nil {
		h.respond(s, i, "Ticket not found.", true)
		return
	}

	closerID := h.getUserID(i)

	if err := h.db.CloseTicket(i.ChannelID, closerID, "closed"); err != nil {
		h.respondError(s, i, "Failed to close ticket")
		return
	}

	// Remove all non-staff members from the channel
	channel, err := s.Channel(i.ChannelID)
	if err == nil && channel.PermissionOverwrites != nil {
		for _, overwrite := range channel.PermissionOverwrites {
			// Only remove member overwrites (not role overwrites)
			if overwrite.Type == discordgo.PermissionOverwriteTypeMember {
				// Don't remove staff role members - check if this is the support role
				if overwrite.ID != h.cfg.SupportRoleID {
					s.ChannelPermissionDelete(i.ChannelID, overwrite.ID)
				}
			}
		}
	}

	// Get category for this ticket
	cat := h.cfg.GetCategory(string(ticket.Type))
	if cat == nil {
		cat = &config.TicketCategory{ID: string(ticket.Type), Name: string(ticket.Type), Emoji: "🎫", Color: 0x5865F2}
	}

	// Build reopen/delete buttons
	buttons := []discordgo.MessageComponent{
		discordgo.Button{Label: "Reopen Ticket", Style: discordgo.SuccessButton, CustomID: "reopen_ticket", Emoji: &discordgo.ComponentEmoji{Name: "🔓"}},
		discordgo.Button{Label: "Delete Ticket", Style: discordgo.DangerButton, CustomID: "delete_ticket", Emoji: &discordgo.ComponentEmoji{Name: "🗑️"}},
	}

	// Update message with closed status while preserving ticket info
	statusText := fmt.Sprintf("🔒 Closed by <@%s>", closerID)
	components := h.buildTicketComponentsV2(ticket, cat, ticket.UserID, ticket.FormData, buttons, statusText)

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Flags:      discordgo.MessageFlagsIsComponentsV2,
			Components: components,
		},
	})
}

func (h *Handler) handleReopenTicket(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !h.isStaff(i) {
		h.respond(s, i, "❌ Only staff members can reopen tickets.", true)
		return
	}

	ticket, err := h.db.GetTicketByChannel(i.ChannelID)
	if err != nil || ticket == nil {
		h.respond(s, i, "Ticket not found.", true)
		return
	}

	// Re-add ticket creator to the channel
	err = s.ChannelPermissionSet(i.ChannelID, ticket.UserID, discordgo.PermissionOverwriteTypeMember,
		discordgo.PermissionViewChannel|discordgo.PermissionSendMessages|discordgo.PermissionReadMessageHistory, 0)
	if err != nil {
		h.respondError(s, i, "Failed to add user back to ticket")
		return
	}

	// Update ticket status back to open
	h.db.ReopenTicket(i.ChannelID)

	staffID := h.getUserID(i)

	// Get category for this ticket
	cat := h.cfg.GetCategory(string(ticket.Type))
	if cat == nil {
		cat = &config.TicketCategory{ID: string(ticket.Type), Name: string(ticket.Type), Emoji: "🎫", Color: 0x5865F2}
	}

	// Build buttons based on ticket type
	var buttons []discordgo.MessageComponent
	if cat.HasApproval {
		buttons = append(buttons, discordgo.Button{Label: "Approve", Style: discordgo.SuccessButton, CustomID: "approve_" + i.ChannelID, Emoji: &discordgo.ComponentEmoji{Name: "✅"}})
		buttons = append(buttons, discordgo.Button{Label: "Deny", Style: discordgo.DangerButton, CustomID: "deny_" + i.ChannelID, Emoji: &discordgo.ComponentEmoji{Name: "❌"}})
	}
	buttons = append(buttons, discordgo.Button{Label: "Close", Style: discordgo.SecondaryButton, CustomID: "close_ticket", Emoji: &discordgo.ComponentEmoji{Name: "🔒"}})

	// Update message with reopened status while preserving ticket info
	statusText := fmt.Sprintf("🔓 Reopened by <@%s>", staffID)
	components := h.buildTicketComponentsV2(ticket, cat, ticket.UserID, ticket.FormData, buttons, statusText)

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Flags:      discordgo.MessageFlagsIsComponentsV2,
			Components: components,
		},
	})
}

func (h *Handler) handleDeleteTicket(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !h.isStaff(i) {
		h.respond(s, i, "❌ Only staff members can delete tickets.", true)
		return
	}

	ticket, err := h.db.GetTicketByChannel(i.ChannelID)
	if err != nil || ticket == nil {
		h.respond(s, i, "Ticket not found.", true)
		return
	}

	channelID := i.ChannelID
	guildID := i.GuildID

	// Get category for this ticket
	cat := h.cfg.GetCategory(string(ticket.Type))
	if cat == nil {
		cat = &config.TicketCategory{ID: string(ticket.Type), Name: string(ticket.Type), Emoji: "🎫", Color: 0x5865F2}
	}

	// Update message with deleting status while preserving ticket info
	statusText := "🗑️ Saving transcript and deleting..."
	components := h.buildTicketComponentsV2(ticket, cat, ticket.UserID, ticket.FormData, []discordgo.MessageComponent{}, statusText)

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Flags:      discordgo.MessageFlagsIsComponentsV2,
			Components: components,
		},
	})

	go func() {
		// Build and save transcript before deleting
		messages, err := s.ChannelMessages(channelID, 100, "", "", "")
		if err == nil && len(messages) > 0 {
			var transcript strings.Builder
			transcript.WriteString(fmt.Sprintf("=== TICKET #%d TRANSCRIPT ===\n", ticket.TicketNumber))
			transcript.WriteString(fmt.Sprintf("Type: %s\n", ticket.Type))
			transcript.WriteString(fmt.Sprintf("User: %s\n", ticket.UserID))
			transcript.WriteString(fmt.Sprintf("Subject: %s\n", ticket.Subject))
			transcript.WriteString(fmt.Sprintf("Created: %s\n", ticket.CreatedAt.Format(time.RFC1123)))
			transcript.WriteString("=============================\n\n")

			// Reverse to chronological order
			for j := len(messages) - 1; j >= 0; j-- {
				msg := messages[j]
				timestamp := msg.Timestamp.Format("2006-01-02 15:04:05")
				transcript.WriteString(fmt.Sprintf("[%s] %s: %s\n", timestamp, msg.Author.Username, msg.Content))
			}

			transcriptContent := transcript.String()
			h.db.SaveTranscript(ticket.ID, channelID, guildID, ticket.UserID, transcriptContent)

			// Post transcript to log channel
			if h.cfg.LogChannelID != "" {
				embed := &discordgo.MessageEmbed{
					Title: fmt.Sprintf("📜 Ticket #%d Transcript", ticket.TicketNumber),
					Color: 0x5865F2,
					Fields: []*discordgo.MessageEmbedField{
						{Name: "Type", Value: string(ticket.Type), Inline: true},
						{Name: "User", Value: fmt.Sprintf("<@%s>", ticket.UserID), Inline: true},
						{Name: "Subject", Value: ticket.Subject, Inline: false},
						{Name: "Created", Value: ticket.CreatedAt.Format(time.RFC1123), Inline: true},
						{Name: "Deleted", Value: time.Now().Format(time.RFC1123), Inline: true},
					},
					Footer: &discordgo.MessageEmbedFooter{Text: "Ticket deleted"},
				}

				// Send transcript as a file attachment
				filename := fmt.Sprintf("ticket-%d-transcript.txt", ticket.TicketNumber)
				s.ChannelMessageSendComplex(h.cfg.LogChannelID, &discordgo.MessageSend{
					Embed: embed,
					Files: []*discordgo.File{
						{
							Name:   filename,
							Reader: bytes.NewReader([]byte(transcriptContent)),
						},
					},
				})
			}
		}

		time.Sleep(2 * time.Second)
		s.ChannelDelete(channelID)
	}()
}

func (h *Handler) handleCancelClose(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ticket, err := h.db.GetTicketByChannel(i.ChannelID)
	if err != nil || ticket == nil {
		h.respond(s, i, "Ticket not found.", true)
		return
	}

	// Get category for this ticket
	cat := h.cfg.GetCategory(string(ticket.Type))
	if cat == nil {
		cat = &config.TicketCategory{ID: string(ticket.Type), Name: string(ticket.Type), Emoji: "🎫", Color: 0x5865F2}
	}

	// Restore original buttons based on ticket type and status
	var buttons []discordgo.MessageComponent
	if cat.HasApproval && ticket.Status == database.StatusOpen {
		buttons = append(buttons, discordgo.Button{Label: "Approve", Style: discordgo.SuccessButton, CustomID: "approve_" + i.ChannelID, Emoji: &discordgo.ComponentEmoji{Name: "✅"}})
		buttons = append(buttons, discordgo.Button{Label: "Deny", Style: discordgo.DangerButton, CustomID: "deny_" + i.ChannelID, Emoji: &discordgo.ComponentEmoji{Name: "❌"}})
	}
	buttons = append(buttons, discordgo.Button{Label: "Close", Style: discordgo.SecondaryButton, CustomID: "close_ticket", Emoji: &discordgo.ComponentEmoji{Name: "🔒"}})

	// Restore original ticket view with default status
	components := h.buildTicketComponentsV2(ticket, cat, ticket.UserID, ticket.FormData, buttons, "")

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Flags:      discordgo.MessageFlagsIsComponentsV2,
			Components: components,
		},
	})
}

func (h *Handler) handleTicketPanel(s *discordgo.Session, i *discordgo.InteractionCreate) {
	perms, _ := s.UserChannelPermissions(i.Member.User.ID, i.ChannelID)
	if perms&discordgo.PermissionManageChannels == 0 {
		h.respond(s, i, "You need Manage Channels permission.", true)
		return
	}

	// Components v2 ticket panel using Container, Section, TextDisplay, and Separator
	accentColor := 0x5865F2 // Blurple
	dividerTrue := true
	spacingLarge := discordgo.SeparatorSpacingSizeLarge

	// Build container components
	containerComponents := []discordgo.MessageComponent{
		// Header section
		discordgo.TextDisplay{Content: "# 🎫 Support Center"},
		discordgo.TextDisplay{Content: "Select the type of ticket you need to create below."},
		// Separator
		discordgo.Separator{Divider: &dividerTrue, Spacing: &spacingLarge},
	}

	// Dynamically add sections for each configured category
	buttonStyles := []discordgo.ButtonStyle{
		discordgo.PrimaryButton,
		discordgo.SecondaryButton,
		discordgo.SuccessButton,
		discordgo.DangerButton,
	}

	for idx, cat := range h.cfg.Categories {
		style := buttonStyles[idx%len(buttonStyles)]
		section := discordgo.Section{
			Accessory: discordgo.Button{
				Label:    cat.Name,
				Style:    style,
				CustomID: "create_ticket_" + cat.ID,
				Emoji:    &discordgo.ComponentEmoji{Name: cat.Emoji},
			},
			Components: []discordgo.MessageComponent{
				discordgo.TextDisplay{Content: fmt.Sprintf("**%s %s**\n%s", cat.Emoji, cat.Name, cat.Description)},
			},
		}
		containerComponents = append(containerComponents, section)
	}

	s.ChannelMessageSendComplex(i.ChannelID, &discordgo.MessageSend{
		Flags: discordgo.MessageFlagsIsComponentsV2,
		Components: []discordgo.MessageComponent{
			discordgo.Container{
				AccentColor: &accentColor,
				Components:  containerComponents,
			},
		},
	})

	h.respond(s, i, "✅ Ticket panel created!", true)
}

func (h *Handler) handleTicketStats(s *discordgo.Session, i *discordgo.InteractionCreate) {
	total, open, closed, err := h.db.GetTicketStats(i.GuildID)
	if err != nil {
		h.respondError(s, i, "Failed to get statistics")
		return
	}

	detailed, _ := h.db.GetDetailedStats(i.GuildID)

	embed := &discordgo.MessageEmbed{
		Title: "📊 Ticket Statistics",
		Color: 0x5865F2,
		Fields: []*discordgo.MessageEmbedField{
			{Name: "Total", Value: fmt.Sprintf("%d", total), Inline: true},
			{Name: "Open", Value: fmt.Sprintf("%d", open), Inline: true},
			{Name: "Closed", Value: fmt.Sprintf("%d", closed), Inline: true},
			{Name: "Support", Value: fmt.Sprintf("%d", detailed["support"]), Inline: true},
			{Name: "Appeals", Value: fmt.Sprintf("%d", detailed["appeal"]), Inline: true},
			{Name: "Applications", Value: fmt.Sprintf("%d", detailed["application"]), Inline: true},
			{Name: "Reports", Value: fmt.Sprintf("%d", detailed["report"]), Inline: true},
		},
	}
	h.respondEmbed(s, i, embed, false)
}

func (h *Handler) handleAddUser(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ticket, err := h.db.GetTicketByChannel(i.ChannelID)
	if err != nil || ticket == nil {
		h.respond(s, i, "This command can only be used in a ticket channel.", true)
		return
	}

	userID := i.ApplicationCommandData().Options[0].UserValue(s).ID
	err = s.ChannelPermissionSet(i.ChannelID, userID, discordgo.PermissionOverwriteTypeMember,
		discordgo.PermissionViewChannel|discordgo.PermissionSendMessages|discordgo.PermissionReadMessageHistory, 0)
	if err != nil {
		h.respondError(s, i, "Failed to add user")
		return
	}
	h.respond(s, i, fmt.Sprintf("✅ <@%s> added to ticket.", userID), false)
}

func (h *Handler) handleRemoveUser(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ticket, err := h.db.GetTicketByChannel(i.ChannelID)
	if err != nil || ticket == nil {
		h.respond(s, i, "This command can only be used in a ticket channel.", true)
		return
	}

	userID := i.ApplicationCommandData().Options[0].UserValue(s).ID
	if userID == ticket.UserID {
		h.respond(s, i, "Cannot remove ticket owner.", true)
		return
	}

	s.ChannelPermissionDelete(i.ChannelID, userID)
	h.respond(s, i, fmt.Sprintf("✅ <@%s> removed from ticket.", userID), false)
}

// logTranscript sends the ticket transcript to the log channel
func (h *Handler) logTranscript(s *discordgo.Session, ticket *database.Ticket, closerID string, transcriptText string) {
	if h.cfg.LogChannelID == "" {
		return
	}

	embed := &discordgo.MessageEmbed{
		Title: fmt.Sprintf("📜 Ticket #%d Transcript", ticket.TicketNumber),
		Color: 0x5865F2,
		Fields: []*discordgo.MessageEmbedField{
			{Name: "Type", Value: string(ticket.Type), Inline: true},
			{Name: "User", Value: fmt.Sprintf("<@%s>", ticket.UserID), Inline: true},
			{Name: "Closed By", Value: fmt.Sprintf("<@%s>", closerID), Inline: true},
			{Name: "Subject", Value: ticket.Subject, Inline: false},
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	// Send embed with transcript as file attachment
	s.ChannelMessageSendComplex(h.cfg.LogChannelID, &discordgo.MessageSend{
		Embeds: []*discordgo.MessageEmbed{embed},
		Files: []*discordgo.File{
			{
				Name:        fmt.Sprintf("transcript-%d.txt", ticket.TicketNumber),
				ContentType: "text/plain",
				Reader:      strings.NewReader(transcriptText),
			},
		},
	})
}

// Helpers
func (h *Handler) getUserID(i *discordgo.InteractionCreate) string {
	if i.Member != nil {
		return i.Member.User.ID
	}
	if i.User != nil {
		return i.User.ID
	}
	return ""
}

func (h *Handler) isStaff(i *discordgo.InteractionCreate) bool {
	if h.cfg.SupportRoleID == "" {
		return true
	}
	if i.Member == nil {
		return false
	}
	for _, role := range i.Member.Roles {
		if role == h.cfg.SupportRoleID {
			return true
		}
	}
	return false
}

func (h *Handler) respond(s *discordgo.Session, i *discordgo.InteractionCreate, content string, ephemeral bool) {
	flags := discordgo.MessageFlags(0)
	if ephemeral {
		flags = discordgo.MessageFlagsEphemeral
	}
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Content: content, Flags: flags},
	})
}

func (h *Handler) respondEmbed(s *discordgo.Session, i *discordgo.InteractionCreate, embed *discordgo.MessageEmbed, ephemeral bool) {
	flags := discordgo.MessageFlags(0)
	if ephemeral {
		flags = discordgo.MessageFlagsEphemeral
	}
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Embeds: []*discordgo.MessageEmbed{embed}, Flags: flags},
	})
}

func (h *Handler) respondWithComponents(s *discordgo.Session, i *discordgo.InteractionCreate, embed *discordgo.MessageEmbed, components []discordgo.MessageComponent) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Embeds: []*discordgo.MessageEmbed{embed}, Components: components, Flags: discordgo.MessageFlagsEphemeral},
	})
}

func (h *Handler) respondError(s *discordgo.Session, i *discordgo.InteractionCreate, msg string) {
	h.respond(s, i, "❌ "+msg, true)
}

func stringPtr(s string) *string { return &s }

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func processLinksWithVideos(text string) (string, []string) {
	if text == "" {
		return "", nil
	}
	var videos, others []string
	urls := urlRegex.FindAllString(text, -1)
	for _, url := range urls {
		if isVideoURL(url) {
			videos = append(videos, url)
		} else {
			others = append(others, fmt.Sprintf("🔗 <%s>", url))
		}
	}
	return strings.Join(others, "\n"), videos
}

func isVideoURL(url string) bool {
	if youtubeRegex.MatchString(url) || twitchClipRegex.MatchString(url) {
		return true
	}
	lower := strings.ToLower(url)
	for _, ext := range []string{".mp4", ".webm", ".mov"} {
		if strings.Contains(lower, ext) {
			return true
		}
	}
	for _, domain := range []string{"tiktok.com", "streamable.com", "medal.tv"} {
		if strings.Contains(url, domain) {
			return true
		}
	}
	return false
}
