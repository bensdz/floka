// pkg/container/container.go
package container

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

// Container represents a running container
type Container struct {
    ID      string
    Image   string
    Command []string
    Status  string
    Pid     int
}

// ContainerOpts contains options for container creation
type ContainerOpts struct {
    Memory    int64 // Memory limit in bytes
    CPUShares int64 // CPU shares (relative weight)
}

// Run creates and starts a new container
func Run(image string, command []string, opts *ContainerOpts) (*Container, error) {
    containerID := generateID()
    
    // Set up container directories relative to the current working directory
    containersDir := "containers"
    containerDir := filepath.Join(containersDir, containerID)
    rootfs := filepath.Join(containerDir, "rootfs")
    
    if err := os.MkdirAll(rootfs, 0755); err != nil {
        return nil, fmt.Errorf("failed to create container filesystem: %w", err)
    }
    
    // Create a simple container structure
    if err := prepareRootfs(rootfs, image); err != nil {
        return nil, fmt.Errorf("failed to prepare rootfs: %w", err)
    }
    
    container := &Container{
        ID:      containerID,
        Image:   image,
        Command: command,
        Status:  "created",
    }
    
    // Save container metadata in consistent location
    metadataDir := filepath.Join(containerDir, "metadata")
    if err := os.MkdirAll(metadataDir, 0755); err != nil {
        return nil, fmt.Errorf("failed to create metadata directory: %w", err)
    }
    
    // Save the initial metadata
    if err := container.updateMetadata(); err != nil {
        return nil, fmt.Errorf("failed to save container metadata: %w", err)
    }
    
    // Set up cgroups if options provided
    if opts != nil {
        if err := setupCgroups(containerID, opts); err != nil {
            return nil, fmt.Errorf("failed to set up cgroups: %w", err)
        }
    }
    
    // Start the container process
    if err := container.start(rootfs); err != nil {
        return container, err
    }
    
    return container, nil
}

// updateMetadata updates the container's metadata file with current state
func (c *Container) updateMetadata() error {
	metadataDir := filepath.Join("containers", c.ID, "metadata")
    if err := os.MkdirAll(metadataDir, 0755); err != nil {
        return fmt.Errorf("failed to create metadata directory: %w", err)
    }
    
    metadataFile := filepath.Join(metadataDir, "container.json")
    
    metadata := map[string]interface{}{
        "ID":      c.ID,
        "Image":   c.Image,
        "Command": c.Command,
        "Status":  c.Status,
        "Pid":     c.Pid,
        "Updated": time.Now().Format(time.RFC3339),
    }
    
    metadataJSON, err := json.Marshal(metadata)
    if err != nil {
        return fmt.Errorf("failed to serialize container metadata: %w", err)
    }
    
    return os.WriteFile(metadataFile, metadataJSON, 0644)
}

// prepareRootfs sets up the root filesystem for the container
func prepareRootfs(rootfs, image string) error {
	// 1. Create the rootfs directory if it doesn't exist
	if err := os.MkdirAll(rootfs, 0755); err != nil {
		return fmt.Errorf("failed to create rootfs: %w", err)
	}

	// 2. Bind mount the image directory to rootfs
	if err := syscall.Mount(image, rootfs, "", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("failed to bind mount image to rootfs: %w %s", err, image)
	}

	// 3. Create standard mount points
	mountPoints := []string{"proc", "sys", "dev", "tmp", "usr/local/bin"}
	for _, dir := range mountPoints {
		path := filepath.Join(rootfs, dir)
		if err := os.MkdirAll(path, 0755); err != nil {
			return fmt.Errorf("failed to create %s in rootfs: %w", dir, err)
		}
	}

	// 4. Copy the floka executable into the container's rootfs
	hostExecutablePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get host executable path: %w", err)
	}

	containerExecutablePath := filepath.Join(rootfs, "usr", "local", "bin", "floka")

	input, err := os.ReadFile(hostExecutablePath)
	if err != nil {
		return fmt.Errorf("failed to read host executable: %w", err)
	}

	if err := os.WriteFile(containerExecutablePath, input, 0755); err != nil {
		return fmt.Errorf("failed to write executable to rootfs: %w", err)
	}


	// mounting these filesystems in the container process itself
	// after unsharing the mount namespace, not here in the parent process

	return nil
}

// setupCgroups configures resource limits for the container
func setupCgroups(containerID string, opts *ContainerOpts) error {
    // Create cgroup directories
    cgroupPath := filepath.Join("/sys/fs/cgroup")
    
    // Check if we're using cgroup v2 unified hierarchy
    isUnifiedCgroupV2 := false
    if _, err := os.Stat(filepath.Join(cgroupPath, "cgroup.controllers")); err == nil {
        isUnifiedCgroupV2 = true
    }
    
    if isUnifiedCgroupV2 {
        // Cgroup v2 approach
        containerCgroupDir := filepath.Join(cgroupPath, "floka", containerID)
        if err := os.MkdirAll(containerCgroupDir, 0755); err != nil {
            return fmt.Errorf("failed to create cgroup directory (v2): %w", err)
        }
        
        // Set memory limit
        if opts.Memory > 0 {
            memMaxPath := filepath.Join(containerCgroupDir, "memory.max")
            if err := os.WriteFile(memMaxPath, []byte(strconv.FormatInt(opts.Memory, 10)), 0644); err != nil {
                return fmt.Errorf("failed to set memory limit: %w", err)
            }
        }
        
        // Set CPU weight
        if opts.CPUShares > 0 {
            cpuWeightPath := filepath.Join(containerCgroupDir, "cpu.weight")
            // Convert Docker-style shares (2-262144) to cgroup v2 weight (1-10000)
            weight := 1 + ((opts.CPUShares - 2) * 9998) / 262142
            if weight < 1 {
                weight = 1
            }
            if weight > 10000 {
                weight = 10000
            }
            
            if err := os.WriteFile(cpuWeightPath, []byte(strconv.FormatInt(weight, 10)), 0644); err != nil {
                return fmt.Errorf("failed to set CPU weight: %w", err)
            }
        }
    } else {
        // Cgroup v1 approach
        subsystems := []string{"memory", "cpu"}
        
        for _, subsystem := range subsystems {
            subsystemPath := filepath.Join(cgroupPath, subsystem, "floka", containerID)
            if err := os.MkdirAll(subsystemPath, 0755); err != nil {
                return fmt.Errorf("failed to create cgroup directory (v1): %w", err)
            }
            
            switch subsystem {
            case "memory":
                if opts.Memory > 0 {
                    // Set memory limit
                    memLimitPath := filepath.Join(subsystemPath, "memory.limit_in_bytes")
                    if err := os.WriteFile(memLimitPath, []byte(strconv.FormatInt(opts.Memory, 10)), 0644); err != nil {
                        return fmt.Errorf("failed to set memory limit: %w", err)
                    }
                }
            case "cpu":
                if opts.CPUShares > 0 {
                    // Set CPU shares
                    cpuSharesPath := filepath.Join(subsystemPath, "cpu.shares")
                    if err := os.WriteFile(cpuSharesPath, []byte(strconv.FormatInt(opts.CPUShares, 10)), 0644); err != nil {
                        return fmt.Errorf("failed to set CPU shares: %w", err)
                    }
                }
            }
        }
    }
    
    return nil
}

// Start the container process
func (c *Container) start(rootfs string) error {
    fmt.Printf("Starting container %s with image %s\n", c.ID, c.Image)
    
    // The path to the executable *inside* the container's chrooted environment
    containerExecutableInternalPath := "/usr/local/bin/floka"
   
    // Use self-exec trick to enter namespaces, using the path inside the container
    cmd := exec.Command(containerExecutableInternalPath)
   
    // Format should be:
    // /usr/local/bin/floka containerize <command> <args>...
    // The rootfs is handled by Chroot in SysProcAttr
    cmd.Args = []string{containerExecutableInternalPath, "containerize"}
    cmd.Args = append(cmd.Args, c.Command...)
   
    // Also pass rootfs as an environment variable as a backup
    // This might not be strictly necessary anymore if chroot works as expected for all cases
    cmd.Env = append(os.Environ(), fmt.Sprintf("FLOKA_ROOTFS=%s", rootfs))
    
    cmd.Stdin = os.Stdin
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    
    // Set up namespaces
    cmd.SysProcAttr = &syscall.SysProcAttr{
        Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID |
        	syscall.CLONE_NEWNS | syscall.CLONE_NEWNET | syscall.CLONE_NEWIPC,
        Chroot: rootfs, // Set the root filesystem for the container
       }
    
    if err := cmd.Start(); err != nil {
        c.Status = "failed"
        // Update metadata with failed status
        if updateErr := c.updateMetadata(); updateErr != nil {
            fmt.Printf("Warning: failed to update container metadata: %s\n", updateErr)
        }
        return fmt.Errorf("failed to start container: %w", err)
    }
    
    c.Pid = cmd.Process.Pid
    c.Status = "running"
    
    // Update metadata with running status and PID
    if err := c.updateMetadata(); err != nil {
        fmt.Printf("Warning: failed to update container metadata: %s\n", err)
    }
    
    // Add process to cgroups
    if err := addProcessToCgroups(c.ID, c.Pid); err != nil {
        fmt.Printf("Warning: failed to add process to cgroups: %s\n", err)
    }
    
    // In a real implementation, we would track the process and handle its lifecycle
    go func() {
        cmd.Wait()
        c.Status = "stopped"
        
        // Update metadata when container stops
        if err := c.updateMetadata(); err != nil {
            fmt.Printf("Warning: failed to update container metadata: %s\n", err)
        }
        
        fmt.Printf("Container %s stopped\n", c.ID)
    }()
    
    return nil
}

// ListContainers returns a list of all containers
func ListContainers() ([]*Container, error) {
	containersDir := "containers"
    if _, err := os.Stat(containersDir); os.IsNotExist(err) {
        // No containers directory exists yet
        return []*Container{}, nil
    }
    
    entries, err := os.ReadDir(containersDir)
    if err != nil {
        return nil, fmt.Errorf("failed to read containers directory: %w", err)
    }
    
    var containers []*Container
    
    for _, entry := range entries {
        if !entry.IsDir() {
            continue
        }
        
        containerID := entry.Name()
        metadataFile := filepath.Join(containersDir, containerID, "metadata", "container.json")
        
        data, err := os.ReadFile(metadataFile)
        if err != nil {
            fmt.Printf("Warning: could not read metadata for container %s: %s\n", containerID, err)
            continue
        }
        
        var metadata map[string]interface{}
        if err := json.Unmarshal(data, &metadata); err != nil {
            fmt.Printf("Warning: could not parse metadata for container %s: %s\n", containerID, err)
            continue
        }
        
        // Extract the command as a string slice
        var cmd []string
        if cmdJSON, ok := metadata["Command"].([]interface{}); ok {
            for _, c := range cmdJSON {
                if str, ok := c.(string); ok {
                    cmd = append(cmd, str)
                }
            }
        }
        
        container := &Container{
            ID:      containerID,
            Image:   metadata["Image"].(string),
            Command: cmd,
            Status:  metadata["Status"].(string),
        }
        
        // Try to get PID if present
        if pid, ok := metadata["Pid"].(float64); ok {
            container.Pid = int(pid)
        }
        
        containers = append(containers, container)
    }
    
    return containers, nil
}

// addProcessToCgroups adds the container process to appropriate cgroups
func addProcessToCgroups(containerID string, pid int) error {
    cgroupPath := filepath.Join("/sys/fs/cgroup")
    pidStr := strconv.Itoa(pid)
    
    // Check if we're using cgroup v2
    isUnifiedCgroupV2 := false
    if _, err := os.Stat(filepath.Join(cgroupPath, "cgroup.controllers")); err == nil {
        isUnifiedCgroupV2 = true
    }
    
    if isUnifiedCgroupV2 {
        // Cgroup v2
        cgroupProcsPath := filepath.Join(cgroupPath, "floka", containerID, "cgroup.procs")
        return os.WriteFile(cgroupProcsPath, []byte(pidStr), 0644)
    } else {
        // Cgroup v1
        subsystems := []string{"memory", "cpu"}
        for _, subsystem := range subsystems {
            cgroupProcsPath := filepath.Join(cgroupPath, subsystem, "floka", containerID, "tasks")
            if err := os.WriteFile(cgroupProcsPath, []byte(pidStr), 0644); err != nil {
                return err
            }
        }
    }
    
    return nil
}

// Stop terminates a running container
func (c *Container) Stop() error {
    fmt.Printf("Stopping container %s\n", c.ID)
    
    if c.Pid > 0 {
        // Send SIGTERM first
        if err := syscall.Kill(c.Pid, syscall.SIGTERM); err != nil {
            // If SIGTERM fails, try SIGKILL
            if err := syscall.Kill(c.Pid, syscall.SIGKILL); err != nil {
                return fmt.Errorf("failed to kill container process: %w", err)
            }
        }
    }
    
    c.Status = "stopped"
    
    // Update metadata with stopped status
    if err := c.updateMetadata(); err != nil {
        fmt.Printf("Warning: failed to update container metadata: %s\n", err)
    }
    
    return nil
}

// Remove deletes a container
func (c *Container) Remove() error {
    fmt.Printf("Removing container %s\n", c.ID)
    
    // Ensure container is stopped
    if c.Status == "running" {
        if err := c.Stop(); err != nil {
            return err
        }
    }
    
    // Clean up cgroups
    if err := cleanupCgroups(c.ID); err != nil {
        fmt.Printf("Warning: failed to clean up cgroups: %s\n", err)
    }
    
    // Remove container filesystem
    containerDir := filepath.Join("containers", c.ID)
    return os.RemoveAll(containerDir)
}

// cleanupCgroups removes the container's cgroup directories
func cleanupCgroups(containerID string) error {
    cgroupPath := filepath.Join("/sys/fs/cgroup")
    
    // Check if we're using cgroup v2
    isUnifiedCgroupV2 := false
    if _, err := os.Stat(filepath.Join(cgroupPath, "cgroup.controllers")); err == nil {
        isUnifiedCgroupV2 = true
    }
    
    if isUnifiedCgroupV2 {
        // Cgroup v2
        return os.RemoveAll(filepath.Join(cgroupPath, "floka", containerID))
    } else {
        // Cgroup v1
        subsystems := []string{"memory", "cpu"}
        for _, subsystem := range subsystems {
            if err := os.RemoveAll(filepath.Join(cgroupPath, subsystem, "floka", containerID)); err != nil {
                return err
            }
        }
    }
    
    return nil
}

// generateID creates a unique container ID
func generateID() string {
    // In a real implementation, we would generate a proper UUID or similar
    return fmt.Sprintf("cont_%d", time.Now().UnixNano())
}