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
	

	// Define command line flags
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s COMMAND [ARG...]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Commands:\n")
		fmt.Fprintf(os.Stderr, "  run         Run a command in a new container\n")
		//fmt.Fprintf(os.Stderr, "  pull        Pull an image from a registry\n")
		fmt.Fprintf(os.Stderr, "  build       Build an image from a Flokafile\n")
		fmt.Fprintf(os.Stderr, "  images      List images\n")
		fmt.Fprintf(os.Stderr, "  ps          List containers\n")
		fmt.Fprintf(os.Stderr, "  help        Show help\n")
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
		fmt.Printf("Running container with image '%s' and command '%s'\n", imageName, strings.Join(cmdArgs, " "))
		
		// Run with parsed options
		runContainerWithOpts(imageName, cmdArgs, *memLimit, *cpuShares)

	
	case "pull":
		if flag.NArg() < 2 {
			fmt.Println("Error: 'pull' requires 1 argument")
			fmt.Println("Usage: floka pull IMAGE[:TAG]")
			os.Exit(1)
		}
		imageName := flag.Arg(1)
		tag := "latest"
		if strings.Contains(imageName, ":") {
			parts := strings.Split(imageName, ":")
			imageName = parts[0]
			tag = parts[1]
		}
		fmt.Printf("Pulling image %s with tag %s...\n", imageName, tag)
		_, err := fimage.Pull(imageName, tag)
		if err != nil {
			fmt.Printf("Error pulling image: %s\n", err)
			os.Exit(1)
		}
	
		
	case "images":
		fmt.Println("REPOSITORY          TAG                 IMAGE ID            PATH")
		//check for folders in the images directory
		imagesDir := "images"
		files, err := os.ReadDir(imagesDir)
		if err != nil {
			fmt.Printf("Error reading images directory: %v\n", err)
			os.Exit(1)
		}
		for _, file := range files {
			if file.IsDir() {
				imageName := file.Name()
				imagePath := filepath.Join(imagesDir, imageName)
				// Check if the image has a tag
				tag := "latest"
				if strings.Contains(imageName, ":") {
					parts := strings.Split(imageName, ":")
					imageName = parts[0]
					tag = parts[1]
				}
				// Get the image ID (using the directory name as a simple ID)
				imageID := imageName
				
				// Display the image information
				displayID := imageID
				if len(imageID) > 12 {
					displayID = imageID[:12]
				}
				fmt.Printf("%-20s %-20s %-20s %s\n", imageName, tag, displayID, imagePath);
			}
		}
		
	case "ps":
		fmt.Println("CONTAINER ID        IMAGE               COMMAND             STATUS              PORTS")
		
		containers, err := container.ListContainers()
		if err != nil {
			fmt.Printf("Error listing containers: %v\n", err)
			os.Exit(1)
		}
		
		for _, cont := range containers {
			// Format command string (truncate if too long)
			cmdStr := strings.Join(cont.Command, " ")
			if len(cmdStr) > 20 {
				cmdStr = cmdStr[:17] + "..."
			}
			
			// Print container info in tabular format
			fmt.Printf("%-20s %-20s %-20s %-20s\n", 
			cont.ID[:12], 
			cont.Image, 
			cmdStr,
			cont.Status)
		}
		
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

	case "containerize":
		// This is an internal command called by the container.Run method
		// It receives the command to run in the containerized environment
		if flag.NArg() < 2 {
			fmt.Println("Error: not enough arguments for containerize")
			fmt.Println("Usage: containerize COMMAND [ARG...]")
			os.Exit(1)
		}
		
		// Get the command to execute
		// The first argument to "containerize" is the command itself,
		// followed by its arguments.
		command := flag.Args()[1:]
		
		// Run the command in the containerized environment
		// rootfs is no longer passed as Chroot is handled by the caller
		runContainerized(command)

	case "help":
		flag.Usage()
		
	default:
		fmt.Printf("Error: unknown command '%s'\n", command)
		flag.Usage()
		os.Exit(1)
	}
}

// func runContainerized(args []string) {
	// 	if len(args) < 2 {
		// 		fmt.Println("Error: not enough arguments for containerize")
		// 		os.Exit(1)
		// 	}

// 	rootfs := args[0]
// 	command := args[1:]

// 	// Set up the mount namespace
// 	if err := setupContainerFilesystem(rootfs); err != nil {
// 		fmt.Printf("Failed to set up container filesystem: %v\n", err)
// 		os.Exit(1)
// 	}

// 	// Make sure we unmount old_root even if something fails
// 	defer func() {
// 		_ = syscall.Unmount("/old_root", syscall.MNT_DETACH)
// 		_ = os.RemoveAll("/old_root") // optional: clean up dir
// 	}()

// 	// Set hostname
// 	_ = syscall.Sethostname([]byte("floka-container"))

// 	// Change to root
// 	if err := os.Chdir("/"); err != nil {
// 		fmt.Printf("Failed to chdir to new root: %v\n", err)
// 		os.Exit(1)
// 	}

// 	// Execute the container command
// 	cmd := exec.Command(command[0], command[1:]...)
// 	cmd.Stdin = os.Stdin
// 	cmd.Stdout = os.Stdout
// 	cmd.Stderr = os.Stderr

// 	if err := cmd.Run(); err != nil {
// 		if exitErr, ok := err.(*exec.ExitError); ok {
// 			os.Exit(exitErr.ExitCode())
// 		} else {
// 			fmt.Printf("Error running container command: %v\n", err)
// 			os.Exit(1)
// 		}
// 	}
// }

// func setupContainerFilesystem(rootfs string) error {
// 	mountPoints := []string{"proc", "sys", "dev", "old_root"}
// 	for _, dir := range mountPoints {
// 		path := filepath.Join(rootfs, dir)
// 		if err := os.MkdirAll(path, 0755); err != nil {
// 			return fmt.Errorf("failed to create %s: %w", path, err)
// 		}
// 	}

// 	// Bind mount the rootfs to itself to make it a mount point
// 	if err := syscall.Mount(rootfs, rootfs, "", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
// 		return fmt.Errorf("bind mount rootfs failed: %w", err)
// 	}

// 	if err := os.Chdir(rootfs); err != nil {
// 		return fmt.Errorf("failed to chdir to rootfs: %w", err)
// 	}

// 	if err := syscall.PivotRoot(".", "old_root"); err != nil {
// 		return fmt.Errorf("pivot_root failed: %w", err)
// 	}

// 	_ = os.Chdir("/") // switch to new root

// 	// Mount basic filesystems
// 	if err := syscall.Mount("proc", "/proc", "proc", 0, ""); err != nil {
// 		return fmt.Errorf("failed to mount /proc: %w", err)
// 	}
// 	if err := syscall.Mount("sysfs", "/sys", "sysfs", 0, ""); err != nil {
// 		return fmt.Errorf("failed to mount /sys: %w", err)
// 	}
// 	if err := syscall.Mount("tmpfs", "/dev", "tmpfs", 0, ""); err != nil {
// 		return fmt.Errorf("failed to mount /dev: %w", err)
// 	}

// 	return nil
// }


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
		
	// Pull the image if needed
	img, err := fimage.Pull(imageName, "latest") // Assuming "latest" if no tag is specified
	if err != nil {
		// Check if the error is because the image is not found
		if strings.Contains(err.Error(), "not found locally") {
			fmt.Printf("Error: Image '%s:latest' not found locally. Please pull or build it first.\n", imageName)
		} else {
			fmt.Printf("Error preparing image: %s\n", err)
		}
		os.Exit(1)
	}
	
	// Create and start the container using the image's RootDir
	cont, err := container.Run(img.RootDir, command, &opts)
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

func runContainerized(command []string) {
	// This function is now running *inside* the chrooted environment.
	// Mount essential filesystems required for most processes.
	// The target directories (/proc, /sys, /dev) should have been created by prepareRootfs.

	mounts := []struct {
		source string
		target string
		fstype string
		flags  uintptr
		data   string
	}{
		{"proc", "/proc", "proc", 0, ""},
		{"sysfs", "/sys", "sysfs", 0, ""},
		{"tmpfs", "/dev", "tmpfs", syscall.MS_NOSUID | syscall.MS_STRICTATIME, "mode=755,size=65536k"},
	}

	for _, m := range mounts {
		if err := syscall.Mount(m.source, m.target, m.fstype, m.flags, m.data); err != nil {
			fmt.Printf("Error mounting %s in container: %v\n", m.target, err)
			// Attempt to clean up previously successful mounts if one fails
			for i := len(mounts) -1 ; i >=0; i-- {
				syscall.Unmount(mounts[i].target, syscall.MNT_DETACH)
			}
			os.Exit(1)
		}
	}

	// Defer unmounts in reverse order of mounting.
	// MNT_DETACH is used for a lazy unmount, useful if resources are busy.
	defer syscall.Unmount("/dev", syscall.MNT_DETACH)
	defer syscall.Unmount("/sys", syscall.MNT_DETACH)
	defer syscall.Unmount("/proc", syscall.MNT_DETACH)
	
	// Create /dev/pts for pseudo-terminals
	devPtsDir := "/dev/pts"
	if err := os.MkdirAll(devPtsDir, 0755); err == nil {
		if err := syscall.Mount("devpts", devPtsDir, "devpts", syscall.MS_NOSUID|syscall.MS_NOEXEC, "newinstance,ptmxmode=0666,mode=0620,gid=5"); err != nil {
			fmt.Printf("Warning: could not mount /dev/pts: %v\n", err)
		} else {
			defer syscall.Unmount(devPtsDir, syscall.MNT_DETACH)
		}
	} else {
		fmt.Printf("Warning: could not create %s directory: %v\n", devPtsDir, err)
	}


	if len(command) == 0 {
		fmt.Println("Error: no command provided to execute in containerized environment")
		os.Exit(1)
	}

	fmt.Printf("Executing '%s' in container\n", strings.Join(command, " "))

	var cmd *exec.Cmd
	if len(command) > 0 && (command[0] == "bash" || strings.HasSuffix(command[0], "/bash")) && len(command) > 0 {
		// If the command is 'bash', try executing it with an absolute path.
		// This helps differentiate PATH issues from execution/linker issues.
		absoluteBashPath := "/bin/bash"
		if _, err := os.Stat(absoluteBashPath); err == nil {
			fmt.Printf("Attempting to execute bash with absolute path: %s\n", absoluteBashPath)
			cmd = exec.Command(absoluteBashPath, command[1:]...)
		} else {
			// Fallback to original command if /bin/bash doesn't exist (should not happen given previous debug)
			fmt.Printf("Warning: %s not found, falling back to command[0]: %s\n", absoluteBashPath, command[0])
			cmd = exec.Command(command[0], command[1:]...)
		}
	} else if len(command) > 0 {
		cmd = exec.Command(command[0], command[1:]...)
	} else {
		// This case should be caught earlier, but as a safeguard:
		fmt.Println("Error: Empty command in runContainerized")
		os.Exit(1)
	}
	
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// Set a minimal default environment.
	cmd.Env = []string{
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"HOME=/",
		"PWD=/",
		"TERM=xterm", // A common terminal type
	}
	
	if err := cmd.Run(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			os.Exit(exitError.ExitCode())
		}
		fmt.Printf("Error executing command in container: %s\n", err)
		os.Exit(1)
	}
}