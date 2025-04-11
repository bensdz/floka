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

	case "test":

	// case "run":
	// 	if flag.NArg() < 2 {
	// 		fmt.Println("Error: 'run' requires at least 1 argument")
	// 		fmt.Println("Usage: floka run [OPTIONS] IMAGE [COMMAND] [ARG...]")
	// 		os.Exit(1)
	// 	}
		
	// 	// Parse run options
	// 	runFlags := flag.NewFlagSet("run", flag.ExitOnError)
	// 	memLimit := runFlags.String("m", "", "Memory limit (e.g., 512m, 1g)")
	// 	cpuShares := runFlags.Int("c", 0, "CPU shares (relative weight)")
		
	// 	// Find where the options end and the image/command begins
	// 	var optArgs []string
	// 	remainingArgs := flag.Args()[1:] // Skip "run"
	// 	imageArgPos := 0
		
	// 	for i, arg := range remainingArgs {
	// 		if !strings.HasPrefix(arg, "-") {
	// 			imageArgPos = i
	// 			break
	// 		}
	// 		optArgs = append(optArgs, arg)
	// 	}
		
	// 	if len(optArgs) > 0 {
	// 		runFlags.Parse(optArgs)
	// 	}
		
	// 	// Extract image and command
	// 	if imageArgPos >= len(remainingArgs) {
	// 		fmt.Println("Error: IMAGE argument required")
	// 		os.Exit(1)
	// 	}
		
	// 	imageName := remainingArgs[imageArgPos]
	// 	var cmdArgs []string
	// 	if imageArgPos+1 < len(remainingArgs) {
	// 		cmdArgs = remainingArgs[imageArgPos+1:]
	// 	}
		
	// 	// Run with parsed options
	// 	runContainerWithOpts(imageName, cmdArgs, *memLimit, *cpuShares)

	
	// case "pull":
	// 	if flag.NArg() < 2 {
	// 		fmt.Println("Error: 'pull' requires 1 argument")
	// 		fmt.Println("Usage: floka pull IMAGE[:TAG]")
	// 		os.Exit(1)
	// 	}
	// 	pullImage(flag.Arg(1))
	
	// case "build":
	// 	buildFlags := flag.NewFlagSet("build", flag.ExitOnError)
	// 	tagFlag := buildFlags.String("t", "", "Name and optionally a tag in the 'name:tag' format")
	// 	fileFlag := buildFlags.String("f", "flokafile", "Name of the Flokafile")
		
	// 	buildFlags.Parse(flag.Args()[1:])
		
	// 	if *tagFlag == "" {
	// 		fmt.Println("Error: 'build' requires a tag")
	// 		fmt.Println("Usage: floka build -t NAME[:TAG] [PATH]")
	// 		os.Exit(1)
	// 	}
		
	// 	path := "."
	// 	if buildFlags.NArg() > 0 {
	// 		path = buildFlags.Arg(0)
	// 	}
		
	// 	buildImage(*fileFlag, path, *tagFlag)
	
	// case "images":
	// 	fmt.Println("REPOSITORY          TAG                 IMAGE ID            SIZE")
	// 	// CHECKME: This is a mock output
	// 	fmt.Println("mock/ubuntu         latest             1234567890abcdef    123MB")	
	// 	fmt.Println("mock/ubuntu         20.04              0987654321fedcba    120MB")
	
	// case "ps":
	// 	fmt.Println("CONTAINER ID        IMAGE               COMMAND             STATUS              PORTS")
	// 	// CHECKME: This is a mock output
	
	// case "help":
	// 	flag.Usage()
	
	default:
		fmt.Printf("Error: unknown command '%s'\n", command)
		flag.Usage()
		os.Exit(1)
	}
}

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

	// Make sure we unmount old_root even if something fails
	defer func() {
		_ = syscall.Unmount("/old_root", syscall.MNT_DETACH)
		_ = os.RemoveAll("/old_root") // optional: clean up dir
	}()

	// Set hostname
	_ = syscall.Sethostname([]byte("floka-container"))

	// Change to root
	if err := os.Chdir("/"); err != nil {
		fmt.Printf("Failed to chdir to new root: %v\n", err)
		os.Exit(1)
	}

	// Execute the container command
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		} else {
			fmt.Printf("Error running container command: %v\n", err)
			os.Exit(1)
		}
	}
}

func setupContainerFilesystem(rootfs string) error {
	mountPoints := []string{"proc", "sys", "dev", "old_root"}
	for _, dir := range mountPoints {
		path := filepath.Join(rootfs, dir)
		if err := os.MkdirAll(path, 0755); err != nil {
			return fmt.Errorf("failed to create %s: %w", path, err)
		}
	}

	// Bind mount the rootfs to itself to make it a mount point
	if err := syscall.Mount(rootfs, rootfs, "", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("bind mount rootfs failed: %w", err)
	}

	if err := os.Chdir(rootfs); err != nil {
		return fmt.Errorf("failed to chdir to rootfs: %w", err)
	}

	if err := syscall.PivotRoot(".", "old_root"); err != nil {
		return fmt.Errorf("pivot_root failed: %w", err)
	}

	_ = os.Chdir("/") // switch to new root

	// Mount basic filesystems
	if err := syscall.Mount("proc", "/proc", "proc", 0, ""); err != nil {
		return fmt.Errorf("failed to mount /proc: %w", err)
	}
	if err := syscall.Mount("sysfs", "/sys", "sysfs", 0, ""); err != nil {
		return fmt.Errorf("failed to mount /sys: %w", err)
	}
	if err := syscall.Mount("tmpfs", "/dev", "tmpfs", 0, ""); err != nil {
		return fmt.Errorf("failed to mount /dev: %w", err)
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


func buildImage(flokafilePath, contextPath, tag string) {
	fullPath := fmt.Sprintf("%s/%s", contextPath, flokafilePath)
	
	// Parse the Flokafile
	Flokafile, err := flokafile.Parse(fullPath)
	if err != nil {
		fmt.Printf("Error parsing Flokafile: %s\n", err)
		os.Exit(1)
	}
	
	// Execute the Flokafile instructions
	if err := Flokafile.Execute(); err != nil {
		fmt.Printf("Error executing Flokafile: %s\n", err)
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