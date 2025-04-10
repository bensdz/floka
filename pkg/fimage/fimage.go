// pkg/image/image.go
package fimage

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Image represents a container image
type Image struct {
	Name    string
	Tag     string
	ID      string
	Size    int64
	Layers  []string
	RootDir string // Path to the extracted rootfs
}

// Pull downloads an image from a registry
func Pull(name string, tag string) (*Image, error) {
	fmt.Printf("Pulling image %s:%s\n", name, tag)
	
	// Create a unique ID for the image
	imageID := generateID()
	
	// Create directories for the image
	imgDir := filepath.Join("/tmp", "floka", "images", imageID)
	rootDir := filepath.Join(imgDir, "rootfs")
	
	if err := os.MkdirAll(rootDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create image directory: %w", err)
	}
	
	// For our simplified implementation, we'll try to use debootstrap if available
	// to create a minimal rootfs for demonstration purposes
	// In a real implementation, we would download the layers from a registry
	
	img := &Image{
		Name:    name,
		Tag:     tag,
		ID:      imageID,
		Size:    0,
		Layers:  []string{},
		RootDir: rootDir,
	}
	
	// Try to detect what kind of image is requested
	if strings.Contains(name, "alpine") {
		if err := createAlpineRootfs(rootDir); err != nil {
			// If we can't create a real rootfs, use a mock one
			fmt.Printf("Warning: Couldn't create Alpine rootfs, using mock: %v\n", err)
			if err := createMockRootfs(rootDir); err != nil {
				return nil, err
			}
		}
	} else if strings.Contains(name, "ubuntu") || strings.Contains(name, "debian") {
		if err := createDebianRootfs(rootDir); err != nil {
			// If we can't create a real rootfs, use a mock one
			fmt.Printf("Warning: Couldn't create Debian rootfs, using mock: %v\n", err)
			if err := createMockRootfs(rootDir); err != nil {
				return nil, err
			}
		}
	} else {
		// For other images, create a mock rootfs
		if err := createMockRootfs(rootDir); err != nil {
			return nil, err
		}
	}
	
	// Calculate the size
	if size, err := dirSize(rootDir); err == nil {
		img.Size = size
	}
	
	return img, nil
}

// createAlpineRootfs attempts to create an Alpine rootfs using apk tools
func createAlpineRootfs(rootDir string) error {
	// Check if we have apk available
	if _, err := exec.LookPath("apk"); err != nil {
		return fmt.Errorf("apk not found: %w", err)
	}
	
	// Create a minimal Alpine rootfs
	cmd := exec.Command("apk", "--root", rootDir, "--no-cache", "--initdb", "add", "alpine-baselayout", "busybox")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	return cmd.Run()
}

// createDebianRootfs attempts to create a Debian rootfs using debootstrap
func createDebianRootfs(rootDir string) error {
	// Check if we have debootstrap available
	if _, err := exec.LookPath("debootstrap"); err != nil {
		return fmt.Errorf("debootstrap not found: %w", err)
	}
	
	// Create a minimal Debian rootfs
	cmd := exec.Command("debootstrap", "--variant=minbase", "stable", rootDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	return cmd.Run()
}

// createMockRootfs creates a minimal mock rootfs with basic directories
func createMockRootfs(rootDir string) error {
	// Create necessary directories
	dirsToCreate := []string{
		"bin", "sbin", "usr/bin", "usr/sbin", "lib",
		"etc", "var", "tmp", "proc", "sys", "dev",
	}
	
	for _, dir := range dirsToCreate {
		if err := os.MkdirAll(filepath.Join(rootDir, dir), 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	
	// Create a simple shell script as /bin/sh
	shPath := filepath.Join(rootDir, "bin", "sh")
	shScript := `#!/bin/sh
echo "Hello from floka container!"
echo "This is a mock shell with limited functionality."
export PS1="floka# "
while true; do
  echo -n "$PS1"
  read cmd
  if [ "$cmd" = "exit" ]; then
    break
  elif [ -z "$cmd" ]; then
    continue
  else
    echo "Mock shell: $cmd"
  fi
done
`
	if err := os.WriteFile(shPath, []byte(shScript), 0755); err != nil {
		return fmt.Errorf("failed to create mock shell: %w", err)
	}
	
	// Create some basic command files
	cmdsToCreate := map[string]string{
		"ls": `#!/bin/sh
echo "bin  dev  etc  lib  proc  sbin  sys  tmp  usr  var"`,
		"echo": `#!/bin/sh
echo "$@"`,
	}
	
	for cmd, content := range cmdsToCreate {
		cmdPath := filepath.Join(rootDir, "bin", cmd)
		if err := os.WriteFile(cmdPath, []byte(content), 0755); err != nil {
			return fmt.Errorf("failed to create command %s: %w", cmd, err)
		}
	}
	
	return nil
}

// dirSize calculates the total size of a directory
func dirSize(path string) (int64, error) {
	var size int64
	
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	
	return size, err
}

// Build creates a new image from a Dockerfile
func Build(dockerfilePath, tag string) (*Image, error) {
	fmt.Printf("Building image from %s with tag %s\n", dockerfilePath, tag)
	
	// Create a unique ID for the image
	imageID := generateID()
	
	// Create directories for the image
	imgDir := filepath.Join("/tmp", "floka", "images", imageID)
	rootDir := filepath.Join(imgDir, "rootfs")
	
	if err := os.MkdirAll(rootDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create image directory: %w", err)
	}
	
	// In a real implementation, we would:
	// 1. Parse the Dockerfile
	// 2. Execute each instruction
	// 3. Create a new layer for each instruction
	
	// For our mock implementation, create a basic rootfs
	if err := createMockRootfs(rootDir); err != nil {
		return nil, err
	}
	
	img := &Image{
		Name:    tag,
		Tag:     "latest",
		ID:      imageID,
		Size:    0,
		Layers:  []string{"base", "app", "config"},
		RootDir: rootDir,
	}
	
	// Calculate the size
	if size, err := dirSize(rootDir); err == nil {
		img.Size = size
	}
	
	return img, nil
}

// Export writes an image to a tar file
func (img *Image) Export(writer io.Writer) error {
	fmt.Printf("Exporting image %s:%s\n", img.Name, img.Tag)
	
	// Check if tar command is available
	if _, err := exec.LookPath("tar"); err == nil {
		// Use system tar command for better performance
		cmd := exec.Command("tar", "-C", img.RootDir, "-cf", "-", ".")
		cmd.Stdout = writer
		return cmd.Run()
	}
	
	// If tar is not available, write a placeholder message
	_, err := writer.Write([]byte(fmt.Sprintf("Mock export of image %s:%s (ID: %s)\n", img.Name, img.Tag, img.ID)))
	return err
}

// Remove deletes an image
func (img *Image) Remove() error {
	fmt.Printf("Removing image %s:%s\n", img.Name, img.Tag)
	
	// Remove the image directory
	imgDir := filepath.Join("/tmp", "floka", "images", img.ID)
	return os.RemoveAll(imgDir)
}

// generateID creates a unique image ID
func generateID() string {
	// In a real implementation, we would generate a proper hash
	return fmt.Sprintf("img_%d", os.Getpid())
}