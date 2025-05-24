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
		// fmt.Printf("Pulling image %s with tag %s...\n", imageName, tag)
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
	
	cont, err := container.Run(img.RootDir, command, &opts) // Get the container object, use := for cont
	if err != nil {
		fmt.Printf("Error running container: %s\n", err)
		// If container.Run failed before fully creating the container object, cont might be nil.
		// container.Run should ideally handle cleanup of its partial work if it errors out.
		// If cont is non-nil here, it means Run returned an error *after* creating the container struct,
		// which might imply a failure during the start phase.
		if cont != nil {
			// Attempt cleanup if container object exists but Run failed during its operation
			// This is a best-effort cleanup.
			// fmt.Printf("Attempting cleanup for partially created/failed container %s\n", cont.ID)
			_ = cont.Remove() // Ignore error from remove here as we're already in an error path
		}
		os.Exit(1)
	}
	
	// Ensure cleanup after the command has run successfully or if a panic occurs
	if cont != nil {
		defer func() {
			// fmt.Printf("Cleaning up container %s...\n", cont.ID) // Optional: log cleanup
			if removeErr := cont.Remove(); removeErr != nil {
				fmt.Printf("Warning: failed to remove container %s: %v\n", cont.ID, removeErr)
			} // else {
				// fmt.Printf("Container %s removed.\n", cont.ID) // Optional: log successful removal
			// }
		}()
	}
	
	// fmt.Printf("Container started: %s (PID: %d)\n", cont.ID, cont.Pid) // This was already commented
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
			for i := len(mounts) - 1; i >= 0; i-- {
				syscall.Unmount(mounts[i].target, syscall.MNT_DETACH)
			}
			os.Exit(1)
		}
	}
	defer syscall.Unmount("/dev", syscall.MNT_DETACH)
	defer syscall.Unmount("/sys", syscall.MNT_DETACH)
	defer syscall.Unmount("/proc", syscall.MNT_DETACH)

	containerHostname := "floka-container"
	if err := syscall.Sethostname([]byte(containerHostname)); err != nil {
		// fmt.Printf("Warning: failed to set hostname to '%s': %v\n", containerHostname, err)
	}

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
		os.Exit(1)
	}

	fmt.Printf("--- DIAGNOSTIC: runContainerized ---\n")
	fmt.Printf("Received command: %v\n", command)

	var cmdToExec string
	var cmdArgsToExec []string
	var err error // Declare err once for os.Stat calls within this scope

	if len(command) > 0 && (command[0] == "bash" || strings.HasSuffix(command[0], "/bash")) {
		absoluteBashPath := "/bin/bash"
		fmt.Printf("Attempting to use absolute path for bash: %s\n", absoluteBashPath)
		if _, err = os.Stat(absoluteBashPath); err == nil { // Use existing err
			fmt.Printf("Found %s, will use it.\n", absoluteBashPath)
			cmdToExec = absoluteBashPath
			if len(command) > 1 {
				cmdArgsToExec = command[1:]
			}
		} else {
			fmt.Printf("Could not stat %s: %v. Falling back to command[0]: %s\n", absoluteBashPath, err, command[0])
			cmdToExec = command[0]
			if len(command) > 1 {
				cmdArgsToExec = command[1:]
			}
		}
	} else if len(command) > 0 {
		cmdToExec = command[0]
		if len(command) > 1 {
			cmdArgsToExec = command[1:]
		}
	} else {
		fmt.Println("CRITICAL ERROR: Empty command in runContainerized")
		os.Exit(1)
	}
	
	fmt.Printf("Final command to exec: %s\n", cmdToExec)
	fmt.Printf("Final arguments to exec: %v\n", cmdArgsToExec)

	bashCheckPath := "/bin/bash"
	bi, statErr := os.Stat(bashCheckPath)
	if statErr != nil {
		fmt.Printf("os.Stat(%s) error: %v\n", bashCheckPath, statErr)
	} else {
		fmt.Printf("os.Stat(%s): Name: %s, Size: %d, Mode: %s, IsDir: %t\n", bashCheckPath, bi.Name(), bi.Size(), bi.Mode(), bi.IsDir())
	}
	
	usrBinBashCheckPath := "/usr/bin/bash"
	ubi, statErr := os.Stat(usrBinBashCheckPath)
	if statErr != nil {
		fmt.Printf("os.Stat(%s) error: %v\n", usrBinBashCheckPath, statErr)
	} else {
		fmt.Printf("os.Stat(%s): Name: %s, Size: %d, Mode: %s, IsDir: %t\n", usrBinBashCheckPath, ubi.Name(), ubi.Size(), ubi.Mode(), ubi.IsDir())
	}

	cmd := exec.Command(cmdToExec, cmdArgsToExec...)
	
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = "/"
	cmd.Env = []string{
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"HOME=/",
		"PWD=/",
		"TERM=xterm",
	}
	fmt.Printf("Environment PATH for exec: %s\n", getPathFromEnv(cmd.Env))
	fmt.Printf("--- END DIAGNOSTIC: runContainerized ---\n")
	
	if err = cmd.Run(); err != nil { // Assign to existing err
		if exitError, ok := err.(*exec.ExitError); ok {
			os.Exit(exitError.ExitCode())
		}
		fmt.Printf("Error executing command in container: %s\n", err)
		os.Exit(1)
	}
}

func getPathFromEnv(env []string) string {
	for _, e := range env {
		if strings.HasPrefix(e, "PATH=") {
			return strings.TrimPrefix(e, "PATH=")
		}
	}
	return "PATH_NOT_FOUND_IN_ENV_ARRAY"
}