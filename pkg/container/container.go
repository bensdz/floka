// pkg/container/container.go
package container

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
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
	
	// Create container root directory
	rootfs := filepath.Join("/tmp", "floka", "containers", containerID, "rootfs")
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

// prepareRootfs sets up the root filesystem for the container
func prepareRootfs(rootfs, image string) error {
	// In a real implementation, we would extract the image layers
	// For now, let's create some basic directories
	dirsToCreate := []string{
		"proc",
		"sys",
		"dev",
		"etc",
		"bin",
		"usr/bin",
		"lib",
	}
	
	for _, dir := range dirsToCreate {
		if err := os.MkdirAll(filepath.Join(rootfs, dir), 0755); err != nil {
			return err
		}
	}
	
	// Copy a minimal set of binaries to the rootfs
	// In a real implementation, this would come from the image
	basicBinaries := []string{"/bin/sh", "/bin/ls", "/bin/echo"}
	for _, bin := range basicBinaries {
		if _, err := os.Stat(bin); err == nil {
			dest := filepath.Join(rootfs, bin)
			// Ensure the directory exists
			if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
				return err
			}
			
			// Copy file content
			input, err := os.ReadFile(bin)
			if err != nil {
				return err
			}
			if err = os.WriteFile(dest, input, 0755); err != nil {
				return err
			}
		}
	}
	
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
	
	// Use self-exec trick to enter namespaces
	cmd := exec.Command("/proc/self/exe")
	cmd.Args = append([]string{"containerize"}, append([]string{rootfs}, c.Command...)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	// Set up namespaces
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | 
			syscall.CLONE_NEWNS | syscall.CLONE_NEWNET | syscall.CLONE_NEWIPC,
	}
	
	if err := cmd.Start(); err != nil {
		c.Status = "failed"
		return fmt.Errorf("failed to start container: %w", err)
	}
	
	c.Pid = cmd.Process.Pid
	c.Status = "running"
	
	// Add process to cgroups
	if err := addProcessToCgroups(c.ID, c.Pid); err != nil {
		fmt.Printf("Warning: failed to add process to cgroups: %s\n", err)
	}
	
	// In a real implementation, we would track the process and handle its lifecycle
	go func() {
		cmd.Wait()
		c.Status = "stopped"
		fmt.Printf("Container %s stopped\n", c.ID)
	}()
	
	return nil
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
	return os.RemoveAll(filepath.Join("/tmp", "floka", "containers", c.ID))
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
	return fmt.Sprintf("cont_%d", os.Getpid())
}