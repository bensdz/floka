// pkg/image/image.go
package fimage

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Image represents a container image
type Image struct {
    Name    string
    Tag     string
    ID      string
    Size    int64
    Layers  []string
    RootDir string // Path to the extracted rootfs
    Created time.Time
}

// Pull downloads an image from a registry or creates a mock image locally
func Pull(name string, tag string) (*Image, error) {
    fmt.Printf("Pulling image %s:%s\n", name, tag)
    
    if tag == "" {
        tag = "latest"
    }
    
    // Set up image directories relative to the current working directory
    imagesDir := "images"
    imageID := generateID()
    imageFullName := fmt.Sprintf("%s:%s", name, tag)
    imageDir := filepath.Join(imagesDir, imageFullName)
    rootDir := filepath.Join(imageDir, "rootfs")
    
    // Check if we already have the image locally
    // First try user home directory
    if _, err := os.Stat(imageDir); err == nil {
        fmt.Printf("Image %s already exists locally\n", imageFullName)
        
        // Load existing image metadata
        size, _ := dirSize(rootDir)
        
        return &Image{
            Name:    name,
            Tag:     tag,
            ID:      imageID,
            Size:    size,
            Layers:  []string{"base"},
            RootDir: rootDir,
            Created: getCreationTime(imageDir),
        }, nil
    }
    
    
    // Create image directory structure
    if err := os.MkdirAll(rootDir, 0755); err != nil {
        return nil, fmt.Errorf("failed to create image directory: %w", err)
    }
    
    return nil, fmt.Errorf("image %s not found locally and pull functionality is not implemented", imageFullName)
}

// saveImageMetadata saves the image metadata to a file
func saveImageMetadata(img *Image, imageDir string) error {
    metadataDir := filepath.Join(imageDir, "metadata")
    if err := os.MkdirAll(metadataDir, 0755); err != nil {
        return fmt.Errorf("failed to create metadata directory: %w", err)
    }
    
    // Write a simple metadata file
    metadataFile := filepath.Join(metadataDir, "image.info")
    content := fmt.Sprintf("Name: %s\nTag: %s\nID: %s\nSize: %d bytes\nCreated: %s\n",
        img.Name, img.Tag, img.ID, img.Size, img.Created.Format(time.RFC3339))
    
    return os.WriteFile(metadataFile, []byte(content), 0644)
}

// GetImagesFromLocalStorage returns all images stored locally
func GetImagesFromLocalStorage() ([]*Image, error) {
    imagesDir := "images"
    if _, err := os.Stat(imagesDir); os.IsNotExist(err) {
        // Images directory doesn't exist yet
        return []*Image{}, nil
    }
    
    // List all directories in the images directory
    entries, err := os.ReadDir(imagesDir)
    if err != nil {
        return nil, fmt.Errorf("failed to read images directory: %w", err)
    }
    
    var images []*Image
    
    for _, entry := range entries {
        if !entry.IsDir() {
            continue
        }
        
        // Parse the image name and tag from the directory name
        fullName := entry.Name()
        name := fullName
        tag := "latest"
        
        if strings.Contains(fullName, ":") {
            parts := strings.Split(fullName, ":")
            name = parts[0]
            if len(parts) > 1 {
                tag = parts[1]
            }
        }
        
        imageDir := filepath.Join(imagesDir, fullName)
        rootDir := filepath.Join(imageDir, "rootfs")
        
        // Check if rootfs exists
        if _, err := os.Stat(rootDir); os.IsNotExist(err) {
            continue
        }
        
        // Get size
        size, _ := dirSize(rootDir)
        
        // Create image object
        img := &Image{
            Name:    name,
            Tag:     tag,
            ID:      fullName, // Use the directory name as ID for display
            Size:    size,
            RootDir: rootDir,
            Created: getCreationTime(imageDir),
        }
        
        images = append(images, img)
    }
    
    return images, nil
}

// getCreationTime gets the creation time of a directory
func getCreationTime(path string) time.Time {
    info, err := os.Stat(path)
    if err != nil {
        return time.Now()
    }
    return info.ModTime()
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
    
    // Set up image directories relative to the current working directory
    imagesDir := "images"
    imageDir := filepath.Join(imagesDir, tag)
    rootDir := filepath.Join(imageDir, "rootfs")
    
    // Check if image already exists
    if _, err := os.Stat(imageDir); err == nil {
        return nil, fmt.Errorf("image %s already exists", tag)
    }
    
    if err := os.MkdirAll(rootDir, 0755); err != nil {
        return nil, fmt.Errorf("failed to create image directory: %w", err)
    }
    
    // Parse and process the Flokafile
    flokafileContent, err := os.ReadFile(flokafilePath)
    if err != nil {
        return nil, fmt.Errorf("failed to read Flokafile: %w", err)
    }

    // Parse Flokafile (line by line for simplicity)
    // Note: Base image creation (previously createMockRootfs) needs to be handled
    // by ensuring the FROM instruction properly sets up a valid rootfs.
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
    
    // Extract name and tag from full tag (name:tag)
    imageName := tag
    imageTag := "latest"
    if strings.Contains(tag, ":") {
        parts := strings.Split(tag, ":")
        imageName = parts[0]
        if len(parts) > 1 {
            imageTag = parts[1]
        }
    }
    
    img := &Image{
        Name:    imageName,
        Tag:     imageTag,
        ID:      generateID(),
        Size:    0,
        Layers:  []string{"base", "app", "config"},
        RootDir: rootDir,
        Created: time.Now(),
    }
    
    // Calculate the size
    if size, err := dirSize(rootDir); err == nil {
        img.Size = size
    }
    
    // Save metadata
    if err := saveImageMetadata(img, imageDir); err != nil {
        return nil, fmt.Errorf("failed to save image metadata: %w", err)
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
    imageDir := filepath.Join("images", fmt.Sprintf("%s:%s", img.Name, img.Tag))
    return os.RemoveAll(imageDir)
}

// generateID creates a unique image ID
func generateID() string {
    // Use timestamp for uniqueness
    return fmt.Sprintf("img_%d", time.Now().UnixNano())
}