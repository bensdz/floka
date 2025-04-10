// cmd/main.go
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/bensdz/floka/pkg/container"
	"github.com/bensdz/floka/pkg/fimage"
	"github.com/bensdz/floka/pkg/flokafile"
)

func main() {
	// Special case for "containerize" command used internally for container process
	if len(os.Args) > 1 && os.Args[1] == "containerize" {
		runContainerized(os.Args[2:])
		return
	}

	// Define command line flags
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s COMMAND [ARG...]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Commands:\n")
		fmt.Fprintf(os.Stderr, "  run         Run a command in a new container\n")
		fmt.Fprintf(os.Stderr, "  pull        Pull an image from a registry\n")
		fmt.Fprintf(os.Stderr, "  build       Build an image from a Flokafile\n")
		fmt.Fprintf(os.Stderr, "  images      List images\n")
		fmt.Fprintf(os.Stderr, "  ps          List containers\n")
		fmt.Fprintf(os.Stderr, "  help        Show help\n")
	}
	
	// Parse command line arguments
	flag.Parse()
	
	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}
	
	command := flag.Arg(0)
	
	switch command {
	case "run":
		if flag.NArg() < 2 {
			fmt.Println("Error: 'run' requires at least 1 argument")
			fmt.Println("Usage: floka run [OPTIONS] IMAGE [COMMAND] [ARG...]")
			os.Exit(1)
		}
		
		// Parse run options
		runFlags := flag.NewFlagSet("run", flag.ExitOnError)
		memLimit := runFlags.String("m", "", "Memory limit (e.g., 512m, 1g)")
		cpuShares := runFlags.Int("c", 0, "CPU shares (relative weight)")
		
		// Find where the options end and the image/command begins
		var optArgs []string
		remainingArgs := flag.Args()[1:] // Skip "run"
		imageArgPos := 0
		
		for i, arg := range remainingArgs {
			if !strings.HasPrefix(arg, "-") {
				imageArgPos = i
				break
			}
			optArgs = append(optArgs, arg)
		}
		
		if len(optArgs) > 0 {
			runFlags.Parse(optArgs)
		}
		
		// Extract image and command
		if imageArgPos >= len(remainingArgs) {
			fmt.Println("Error: IMAGE argument required")
			os.Exit(1)
		}
		
		imageName := remainingArgs[imageArgPos]
		var cmdArgs []string
		if imageArgPos+1 < len(remainingArgs) {
			cmdArgs = remainingArgs[imageArgPos+1:]
		}
		
		// Run with parsed options
		runContainerWithOpts(imageName, cmdArgs, *memLimit, *cpuShares)
	
	case "pull":
		if flag.NArg() < 2 {
			fmt.Println("Error: 'pull' requires 1 argument")
			fmt.Println("Usage: floka pull IMAGE[:TAG]")
			os.Exit(1)
		}
		pullImage(flag.Arg(1))
	
	case "build":
		buildFlags := flag.NewFlagSet("build", flag.ExitOnError)
		tagFlag := buildFlags.String("t", "", "Name and optionally a tag in the 'name:tag' format")
		fileFlag := buildFlags.String("f", "Dockerfile", "Name of the Dockerfile")
		
		buildFlags.Parse(flag.Args()[1:])
		
		if *tagFlag == "" {
			fmt.Println("Error: 'build' requires a tag")
			fmt.Println("Usage: floka build -t NAME[:TAG] [PATH]")
			os.Exit(1)
		}
		
		path := "."
		if buildFlags.NArg() > 0 {
			path = buildFlags.Arg(0)
		}
		
		buildImage(*fileFlag, path, *tagFlag)
	
	case "images":
		fmt.Println("REPOSITORY          TAG                 IMAGE ID            SIZE")
		fmt.Println("mock/ubuntu         latest              abc123def456        120 MB")
		fmt.Println("mock/alpine         latest              789ghi101112        5 MB")
	
	case "ps":
		fmt.Println("CONTAINER ID        IMAGE               COMMAND             STATUS              PORTS")
		fmt.Println("cont_12345          mock/ubuntu         \"/bin/bash\"         running             -")
	
	case "help":
		flag.Usage()
	
	default:
		fmt.Printf("Error: unknown command '%s'\n", command)
		flag.Usage()
		os.Exit(1)
	}
}

// runContainerized is called when we are inside the container namespaces
func runContainerized(args []string) {
	if len(args) < 2 {
		fmt.Println("Error: not enough arguments for containerize")
		os.Exit(1)
	}
	
	rootfs := args[0]
	command := args[1:]
	
	// Set up the mount namespace
	if err := setupContainerFilesystem(rootfs); err != nil {
		fmt.Printf("Failed to set up container filesystem: %v\n", err)
		os.Exit(1)
	}
	
	// Change to the new root
	if err := os.Chdir("/"); err != nil {
		fmt.Printf("Failed to change directory to root: %v\n", err)
		os.Exit(1)
	}
	
	// Execute the container command
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	// Set hostname
	syscall.Sethostname([]byte("floka-container"))
	
	if err := cmd.Run(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			os.Exit(exitError.ExitCode())
		} else {
			fmt.Printf("Error running container command: %v\n", err)
			os.Exit(1)
		}
	}
}

// setupContainerFilesystem sets up the mount namespace for the container
func setupContainerFilesystem(rootfs string) error {
	// Mount proc filesystem
	if err := syscall.Mount("proc", filepath.Join(rootfs, "proc"), "proc", 0, ""); err != nil {
		return fmt.Errorf("failed to mount proc: %w", err)
	}
	
	// Mount sysfs
	if err := syscall.Mount("sysfs", filepath.Join(rootfs, "sys"), "sysfs", 0, ""); err != nil {
		return fmt.Errorf("failed to mount sysfs: %w", err)
	}
	
	// Mount /dev as tmpfs
	if err := syscall.Mount("tmpfs", filepath.Join(rootfs, "dev"), "tmpfs", 0, ""); err != nil {
		return fmt.Errorf("failed to mount tmpfs for /dev: %w", err)
	}
	
	// Pivot to the new root
	// This is a simplified approach - a full implementation would use pivot_root
	if err := syscall.Chroot(rootfs); err != nil {
		return fmt.Errorf("failed to chroot: %w", err)
	}
	
	return nil
}

// runContainerWithOpts runs a container with the specified resource options
func runContainerWithOpts(imageName string, command []string, memLimit string, cpuShares int) {
	var opts container.ContainerOpts
	
	// Parse memory limit (e.g., "512m", "1g")
	if memLimit != "" {
		bytes, err := parseMemoryLimit(memLimit)
		if err != nil {
			fmt.Printf("Error parsing memory limit: %s\n", err)
			os.Exit(1)
		}
		opts.Memory = bytes
	}
	
	// Set CPU shares
	if cpuShares > 0 {
		opts.CPUShares = int64(cpuShares)
	}
	
	// If no command specified, use default
	if len(command) == 0 {
		command = []string{"/bin/sh"}
	}
	
	fmt.Printf("Running container with image '%s' and command '%s'\n", 
		imageName, strings.Join(command, " "))
	
	// Pull the image if needed
	img, err := fimage.Pull(imageName, "latest")
	if err != nil {
		fmt.Printf("Error pulling image: %s\n", err)
		os.Exit(1)
	}
	
	// Create and start the container
	cont, err := container.Run(img.Name, command, &opts)
	if err != nil {
		fmt.Printf("Error running container: %s\n", err)
		os.Exit(1)
	}
	
	fmt.Printf("Container started: %s (PID: %d)\n", cont.ID, cont.Pid)
}

// parseMemoryLimit parses a human-readable memory limit to bytes
func parseMemoryLimit(limit string) (int64, error) {
	limit = strings.ToLower(limit)
	
	var multiplier int64 = 1
	
	if strings.HasSuffix(limit, "k") {
		multiplier = 1024
		limit = limit[:len(limit)-1]
	} else if strings.HasSuffix(limit, "m") {
		multiplier = 1024 * 1024
		limit = limit[:len(limit)-1]
	} else if strings.HasSuffix(limit, "g") {
		multiplier = 1024 * 1024 * 1024
		limit = limit[:len(limit)-1]
	}
	
	value, err := strconv.ParseInt(limit, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid memory limit format: %s", limit)
	}
	
	return value * multiplier, nil
}

func pullImage(nameWithTag string) {
	parts := strings.SplitN(nameWithTag, ":", 2)
	name := parts[0]
	tag := "latest"
	
	if len(parts) > 1 {
		tag = parts[1]
	}
	
	fmt.Printf("Pulling image '%s:%s'\n", name, tag)
	
	img, err := fimage.Pull(name, tag)
	if err != nil {
		fmt.Printf("Error pulling image: %s\n", err)
		os.Exit(1)
	}
	
	fmt.Printf("Image pulled: %s\n", img.ID)
}

func buildImage(flokafilePath, contextPath, tag string) {
	fullPath := fmt.Sprintf("%s/%s", contextPath, flokafilePath)
	
	// Parse the Dockerfile
	dockerfile, err := flokafile.Parse(fullPath)
	if err != nil {
		fmt.Printf("Error parsing Dockerfile: %s\n", err)
		os.Exit(1)
	}
	
	// Execute the Dockerfile instructions
	if err := dockerfile.Execute(); err != nil {
		fmt.Printf("Error executing Dockerfile: %s\n", err)
		os.Exit(1)
	}
	
	// Build the image
	img, err := fimage.Build(fullPath, tag)
	if err != nil {
		fmt.Printf("Error building image: %s\n", err)
		os.Exit(1)
	}
	
	fmt.Printf("Image built: %s:%s\n", img.Name, img.Tag)
}