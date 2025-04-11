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

	img := &Image{
		Name:   name,
		Tag:    tag,
		ID:     generateID(),
		Size:   1024 * 1024 * 256, // Mock size
		Layers: []string{"base"},
	}
	
	if strings.Contains(name, "ubuntu") {
        
		// Check if we have the image already
        imgDir := filepath.Join(os.Getenv("HOME"), "floka", "images", "ubuntu")
        if _, err := os.Stat(imgDir); os.IsNotExist(err) {
            return nil, fmt.Errorf("ubuntu image not found at %s - please download it first", imgDir)
        }
        
        return img, nil
	} else {
		// For other images, create a mock filesystem
		fmt.Printf("Creating mock filesystem for %s:%s\n", name, tag)

		// Set the rootfs directory for the image
		img.RootDir = rootDir

		// Create a basic mock rootfs
		if err := createMockRootfs(rootDir); err != nil {
			return nil, fmt.Errorf("failed to create mock rootfs: %w", err)
		}

		// Customize based on the image name
		if strings.Contains(name, "alpine") {
			// Add alpine-specific files
			alpineRelease := filepath.Join(rootDir, "etc", "alpine-release")
			if err := os.WriteFile(alpineRelease, []byte("3.14.0\n"), 0644); err != nil {
				return nil, fmt.Errorf("failed to create alpine-release file: %w", err)
			}
		} else if strings.Contains(name, "debian") {
			// Add debian-specific files
			osRelease := filepath.Join(rootDir, "etc", "os-release")
			if err := os.WriteFile(osRelease, []byte("PRETTY_NAME=\"Debian GNU/Linux 11 (bullseye)\"\n"), 0644); err != nil {
				return nil, fmt.Errorf("failed to create os-release file: %w", err)
			}
		} else {
			// Generic Linux
			osRelease := filepath.Join(rootDir, "etc", "os-release")
			if err := os.WriteFile(osRelease, []byte("PRETTY_NAME=\"Generic Linux\"\n"), 0644); err != nil {
				return nil, fmt.Errorf("failed to create os-release file: %w", err)
			}
		}
	}
	
	// Set the root directory for the image
	img.RootDir = rootDir
	// Calculate the size of the rootfs
	if size, err := dirSize(rootDir); err == nil {
		img.Size = size
	} else {
		return nil, fmt.Errorf("failed to calculate rootfs size: %w", err)
	}
	
	return img, nil
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

// Build creates a new image from a flokafile
func Build(flokafilePath, tag string) (*Image, error) {
	fmt.Printf("Building image from %s with tag %s\n", flokafilePath, tag)
	
	// Create a unique ID for the image
	imageID := generateID()
	
	// Create directories for the image
	imgDir := filepath.Join("/tmp", "floka", "images", imageID)
	rootDir := filepath.Join(imgDir, "rootfs")
	
	if err := os.MkdirAll(rootDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create image directory: %w", err)
	}
	
	// Parse and process the Flokafile
	flokafileContent, err := os.ReadFile(flokafilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read Flokafile: %w", err)
	}

	// Create a basic rootfs structure first
	if err := createMockRootfs(rootDir); err != nil {
		return nil, fmt.Errorf("failed to create base rootfs: %w", err)
	}

	// Parse Flokafile (line by line for simplicity)
	lines := strings.Split(string(flokafileContent), "\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue // Skip empty lines and comments
		}

		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 2 {
			return nil, fmt.Errorf("invalid instruction at line %d: %s", i+1, line)
		}

		instruction, args := parts[0], parts[1]
		
		fmt.Printf("Executing instruction: %s %s\n", instruction, args)
		
		// Process each instruction type
		switch strings.ToUpper(instruction) {
		case "FROM":
			// Get base image - we already have createMockRootfs for now
			fmt.Printf("Using %s as base image\n", args)
			
		case "RUN":
			// Simulate running a command
			fmt.Printf("Running command: %s\n", args)
			// In a real implementation, we would execute the command in a container
			
		case "COPY":
			// Copy files from host to the image
			copyParts := strings.SplitN(args, " ", 2)
			if len(copyParts) != 2 {
				return nil, fmt.Errorf("invalid COPY instruction at line %d", i+1)
			}
			src, dest := copyParts[0], copyParts[1]
			
			// Make sure destination directory exists
			destDir := filepath.Join(rootDir, filepath.Dir(dest))
			if err := os.MkdirAll(destDir, 0755); err != nil {
				return nil, fmt.Errorf("failed to create destination directory: %w", err)
			}
			
			// Copy the file (simplified)
			srcFile, err := os.Open(src)
			if err != nil {
				return nil, fmt.Errorf("failed to open source file: %w", err)
			}
			defer srcFile.Close()
			
			destFile, err := os.Create(filepath.Join(rootDir, dest))
			if err != nil {
				return nil, fmt.Errorf("failed to create destination file: %w", err)
			}
			defer destFile.Close()
			
			if _, err := io.Copy(destFile, srcFile); err != nil {
				return nil, fmt.Errorf("failed to copy file: %w", err)
			}
			
		case "ENV":
			// Set environment variable
			envParts := strings.SplitN(args, "=", 2)
			if len(envParts) != 2 {
				return nil, fmt.Errorf("invalid ENV instruction at line %d", i+1)
			}
			key, value := envParts[0], envParts[1]
			
			// Add to /etc/environment (simplified)
			envFile := filepath.Join(rootDir, "etc", "environment")
			f, err := os.OpenFile(envFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				return nil, fmt.Errorf("failed to open environment file: %w", err)
			}
			if _, err := fmt.Fprintf(f, "%s=%s\n", key, value); err != nil {
				f.Close()
				return nil, fmt.Errorf("failed to write environment variable: %w", err)
			}
			f.Close()
			
		default:
			return nil, fmt.Errorf("unknown instruction at line %d: %s", i+1, instruction)
		}
		
		// In a real implementation, we would commit a new layer here
		fmt.Printf("Committed layer for instruction: %s\n", instruction)
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