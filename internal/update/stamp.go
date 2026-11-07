package update

import (
	"os"
	"path/filepath"
	"time"
)

const (
	checkInterval = 24 * time.Hour
	stampName     = "last-update-check"
	logName       = "update.log"
	// privateMode keeps the stamp and the log readable by the user alone: an update error can
	// name accounts, repositories and URLs.
	privateMode = 0o600
)

func IsDue(stateDir string) bool {
	info, err := os.Stat(filepath.Join(stateDir, stampName))
	if err != nil {
		return true
	}
	return time.Since(info.ModTime()) >= checkInterval
}

func MarkChecked(stateDir string) error {
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return err
	}
	return writePrivate(filepath.Join(stateDir, stampName), []byte(time.Now().UTC().Format(time.RFC3339)))
}

func WriteLog(stateDir string, message string) {
	_ = os.MkdirAll(stateDir, 0o755)
	line := time.Now().UTC().Format(time.RFC3339) + " " + message + "\n"
	_ = writePrivate(filepath.Join(stateDir, logName), []byte(line))
}

// writePrivate writes path with privateMode. os.WriteFile applies the mode only when it creates
// the file, so a file an older ghx left world-readable is tightened here too.
func writePrivate(path string, content []byte) error {
	if err := os.WriteFile(path, content, privateMode); err != nil {
		return err
	}
	return os.Chmod(path, privateMode)
}
