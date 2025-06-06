# Floka: A Simple Containerization Tool

[Floka Logo](photos/Floka.png)

Floka is a basic containerization tool written in Go. It aims to provide a simplified, educational implementation of container concepts like image management, container lifecycle, and namespacing.

[Floka Demo](photos/flokasc.png)

**Note:** This project is currently under development and primarily for learning purposes. Many features are simplified or not yet implemented.

## Core Concepts

*   **Images (`pkg/fimage`):** Floka manages container images. Currently, it simulates image pulling and building. For local testing, image filesystems (like an Ubuntu rootfs) need to be manually placed in the `images/<image_name>:<tag>/rootfs/` directory.
*   **Containers (`pkg/container`):** Floka can run commands within isolated container environments. It uses Linux namespaces (UTS, PID, Mount, Network, IPC) and `chroot` to achieve isolation. The hostname inside the container is set to "floka-container".
*   **CLI (`cmd/main.go`):** A command-line interface is provided to interact with Floka.

## Current Functionality

*   **`floka run <image> [command] [args...]`**:
    *   "Pulls" an image (currently expects it to be locally available in `images/<image>:latest/rootfs/`).
    *   Creates a new container with a unique ID and stores metadata.
    *   Sets up namespaces (UTS, PID, Mount, Network, IPC) and chroots into the image's root filesystem.
    *   Sets the container's hostname to "floka-container".
    *   Executes the specified command (or `/bin/sh` by default) within the container. The main `floka` process waits for this command to complete.
*   **`floka images`**: Lists locally available "images" by scanning the `images/` directory.
*   **`floka ps`**: Lists running/stopped containers by reading metadata from the `containers/` directory.
*   **`floka pull <image>[:<tag>]`**: Simulates pulling. If the image directory `images/<image>:<tag>` exists, it's considered pulled. Otherwise, it creates the directory structure and reports that pull functionality is not implemented.
*   **`floka build -t <tag> [path_to_flokafile_dir]`**: A very basic implementation that can parse a `Flokafile` with `FROM`, `RUN`, `COPY`, and `ENV` instructions. It simulates these operations and creates an image structure in the `images/` directory.

## Project Structure

*   `cmd/main.go`: The main application entry point and CLI handler.
*   `pkg/container/container.go`: Logic for container creation, starting, stopping, and managing namespaces/cgroups.
*   `pkg/fimage/fimage.go`: Logic for image "pulling", "building", and listing.
*   `pkg/flokafile/flokafile.go`: (If it exists, or planned) Logic for parsing Flokafile build instructions.
*   `images/`: Default directory where local image filesystems are stored (e.g., `images/ubuntu:latest/rootfs/`).
*   `containers/`: Default directory where runtime container data (rootfs mounts, metadata) is stored.

## How it Works (Simplified `run` command)

1.  **Host (`floka run ...`):**
    *   The `floka` binary is executed on the host.
    *   The `run` command is parsed.
    *   `fimage.Pull()` checks for the local image directory (e.g., `images/ubuntu:latest/rootfs/`). **Crucially, this directory must be manually populated with the desired image's complete filesystem (including dynamic linker and libraries) for current local testing.**
    *   `container.Run()` (which calls `container.start()`):
        *   Creates a unique directory for the container (e.g., `containers/cont_XYZ/`).
        *   Creates `containers/cont_XYZ/rootfs/`.
        *   Copies the `floka` executable itself into `containers/cont_XYZ/rootfs/usr/local/bin/floka`.
        *   Bind-mounts the source image directory (e.g., `images/ubuntu:latest/rootfs/`) onto `containers/cont_XYZ/rootfs/`.
        *   Re-executes `/usr/local/bin/floka` (the one inside the container's future root) with the `containerize` argument and the user's command (e.g., `bash`). This re-execution uses `syscall.SysProcAttr` to set `Cloneflags` (for new namespaces) and `Chroot` (to `containers/cont_XYZ/rootfs/`).
        *   The `container.start()` function then waits for this re-executed `floka containerize` process to complete.

2.  **Inside Container Setup (`floka containerize ...`):**
    *   The `floka` program starts again, but now it's inside the chrooted and namespaced environment.
    *   The `main` function sees the `containerize` command.
    *   `runContainerized()` is called:
        *   Sets the container hostname to "floka-container" using `syscall.Sethostname()`.
        *   Mounts essential virtual filesystems like `/proc`, `/sys`, `/dev` inside the new root.
        *   Sets basic environment variables like `PATH` and sets the working directory to `/`.
        *   Finally, uses `exec.Command()` to run the user's intended command (e.g., `bash` or `/bin/bash`).

## Setup for Local Development & Testing

1.  **Go Environment:** Ensure you have Go installed and configured.
2.  **Root Privileges:** Running containers typically requires `sudo` due to operations like `mount`, `chroot`, and namespace manipulation.
3.  **Populate Local Images:**
    *   Create the directory structure: `mkdir -p images/ubuntu:latest/rootfs`
    *   Obtain a **complete** Ubuntu root filesystem (e.g., from a Docker export: `docker export $(docker create ubuntu:latest) | tar -C images/ubuntu:latest/rootfs -xf -`). This must include `/bin`, `/etc`, `/usr`, `/lib`, `/lib64` (for 64-bit systems, containing the dynamic linker like `ld-linux-x86-64.so.2`), etc.
    *   Copy the *entire contents* of this Ubuntu rootfs into your `images/ubuntu:latest/rootfs/` directory.
4.  **Build and Run:**
    ```bash
    # Navigate to the project root
    cd /path/to/floka

    # Run a command to see the container's environment
    sudo go run cmd/main.go run ubuntu bash -c "echo 'Inside container:'; hostname; id; ps aux"

    # Example of just running bash (it will exit if non-interactive)
    sudo go run cmd/main.go run ubuntu bash
    ```

## Current Known Issues & Limitations

*   **Image Pulling:** Actual downloading and layer extraction from a registry is not implemented. Relies on manually populated local directories.
*   **Interactive Shells (PTY):** Proper pseudo-terminal (PTY) allocation for fully interactive shells is not implemented. Running `bash` alone will execute non-interactively.
*   **Networking:** While a new network namespace is created, detailed network setup (like veth pairs, bridges) is not implemented. Containers will have isolated loopback but no external connectivity by default.
*   **Security:** Many security aspects of production container runtimes are not implemented. This tool is for educational purposes.
*   **Error Handling:** Can be improved.
*   **Resource Limits (Cgroups):** Basic cgroup setup for memory and CPU shares is present but might need refinement for different cgroup versions and more complex configurations.
*   **Volume Mounts:** Not implemented.
*   **Port Mapping:** Not implemented.

## Future Development Ideas

*   Implement actual image pulling from a Docker registry (parsing manifests, downloading layers, extracting tarballs).
*   Proper PTY allocation for interactive shells.
*   Proper networking setup for containers.
*   Support for volume mounts.
*   More robust Flokafile parsing and execution.
*   Snapshotting/layering for image builds.