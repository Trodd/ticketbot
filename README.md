# Discord Ticket Bot

A feature-rich Discord ticket bot similar to appeal.gg, built with Go and discordgo.

## Features

### 🎫 Multiple Ticket Types

- **Support** - General help and questions
- **Appeal** - Ban/punishment appeals with custom form
- **Application** - Staff applications with detailed questions
- **Report** - Report users or issues

### 👨‍💼 Staff Management

- **Priority Levels** - Set ticket priority (Urgent/High/Normal/Low)
- **Staff Notes** - Internal notes visible only to staff
- **Queue View** - See all open tickets sorted by priority

### 📋 Appeal System

- Custom modal forms for appeals
- Approve/Deny workflow with feedback
- Auto-close after decision
- Full audit logging

### 📝 Application System

- Detailed application forms
- Experience, availability, and motivation questions
- Approve/Deny with reasons

### 🚨 Report System

- Report users or issues
- Evidence/link support
- Priority-based handling

### 🔒 Moderation Tools

- **Blacklist** - Block users from creating tickets
- **User History** - View a user's past tickets and outcomes
- **Transcripts** - Auto-save chat logs when closing

### 📊 Statistics

- Total/Open/Closed counts
- Breakdown by ticket type

## Commands

### User Commands

| Command | Description |
|---------|-------------|
| `/ticket` | Create a support ticket |
| `/appeal` | Submit a ban/punishment appeal |
| `/apply` | Submit a staff application |
| `/report` | Report a user or issue |
| `/close` | Close the current ticket |

### Staff Commands

| Command | Description |
|---------|-------------|
| `/approve` | Approve appeal/application |
| `/deny [reason]` | Deny appeal/application |
| `/priority <level>` | Set ticket priority |
| `/note <content>` | Add internal staff note |
| `/notes` | View all staff notes |
| `/history <user>` | View user's ticket history |
| `/blacklist add <user> <reason>` | Blacklist a user |
| `/blacklist remove <user>` | Remove from blacklist |
| `/transcript` | Save ticket transcript |
| `/queue` | View open ticket queue |

### Admin Commands

| Command | Description |
|---------|-------------|
| `/ticketpanel` | Create ticket panel with buttons |
| `/ticketstats` | View detailed statistics |
| `/adduser <user>` | Add user to ticket |
| `/removeuser <user>` | Remove user from ticket |

## Setup

### Prerequisites

- Go 1.21 or later
- A Discord bot token

### Installation

1. Copy `.env.example` to `.env`:

   ```bash
   cp .env.example .env
   ```

2. Configure your `.env`:

   ```env
   DISCORD_TOKEN=your_bot_token_here
   GUILD_ID=your_server_id
   TICKET_CATEGORY_ID=category_for_tickets
   SUPPORT_ROLE_ID=staff_role_id
   LOG_CHANNEL_ID=logging_channel_id
   ```

3. Install dependencies:

   ```bash
   go mod tidy
   ```

4. Build and run:

   ```bash
   go build -o ticketbot.exe .
   ./ticketbot.exe
   ```

### Bot Permissions

Required permissions:

- Manage Channels
- Send Messages
- Embed Links
- Read Message History
- Use Slash Commands
- Manage Roles

Recommended permission integer: `414464739392`

## Configuration

| Variable | Description | Required |
|----------|-------------|----------|
| `DISCORD_TOKEN` | Bot token | Yes |
| `GUILD_ID` | Server ID for commands | No |
| `TICKET_CATEGORY_ID` | Category for ticket channels | No |
| `SUPPORT_ROLE_ID` | Staff role ID | No |
| `LOG_CHANNEL_ID` | Logging channel | No |

## Project Structure

```
ticketbot/
├── main.go              # Entry point
├── config/
│   └── config.go        # Configuration
├── database/
│   └── database.go      # SQLite operations
├── handlers/
│   └── handlers.go      # Discord handlers
├── go.mod
├── .env.example
└── README.md
```

## Database Tables

- **tickets** - All ticket data with type, status, form data
- **blacklist** - Blocked users
- **transcripts** - Saved chat logs
- **staff_notes** - Internal notes

## License

MIT License
