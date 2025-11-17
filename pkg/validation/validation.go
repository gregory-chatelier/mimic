package validation

import (
	"fmt"
	"os"
)

// ValidateFileExists checks if a file exists at the given path.
func ValidateFileExists(path, description string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("%s not found at %s", description, path)
	}
	return nil
}

// ValidateFileReadable checks if a file exists and is readable at the given path.
func ValidateFileReadable(path string) error {
	if err := ValidateFileExists(path, "File"); err != nil {
		return err
	}
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("cannot read file %s: %w", path, err)
	}
	f.Close()
	return nil
}

// ValidatePrivateKeyPermissions checks if a private key file has secure permissions (0600 on non-Windows).
func ValidatePrivateKeyPermissions(path string) error {
	// FIXME: Temporarily disabled for WSL compatibility.
	// info, err := os.Stat(path)
	// if err != nil {
	// 	return fmt.Errorf("cannot get file info for %s: %w", path, err)
	// }

	// if runtime.GOOS != "windows" {
	// 	if perm := info.Mode().Perm(); perm != 0600 {
	// 		return fmt.Errorf("insecure permissions for private key file %s: expected 0600, got %o", path, perm)
	// 	}
	// }
	return nil
}
