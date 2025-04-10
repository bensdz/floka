// pkg/dockerfile/parser.go
package flokafile

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Instruction represents a single Dockerfile instruction
type Instruction struct {
	Command string
	Args    string
}

// Dockerfile represents a parsed Dockerfile
type Dockerfile struct {
	Instructions []Instruction
	BasePath     string
}

// Parse reads a Dockerfile and returns its instructions
func Parse(path string) (*Dockerfile, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open Dockerfile: %w", err)
	}
	defer file.Close()
	
	dockerfile := &Dockerfile{
		BasePath: path,
	}
	
	scanner := bufio.NewScanner(file)
	var currentInstruction *Instruction
	
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		
		// Check if it's a continuation line (ends with \)
		if currentInstruction != nil && strings.HasSuffix(line, "\\") {
			// Remove the backslash and add to current args
			line = strings.TrimSuffix(line, "\\")
			currentInstruction.Args += " " + strings.TrimSpace(line)
			continue
		}
		
		// If we have a pending instruction, add it
		if currentInstruction != nil {
			dockerfile.Instructions = append(dockerfile.Instructions, *currentInstruction)
			currentInstruction = nil
		}
		
		// Parse new instruction
		parts := strings.SplitN(line, " ", 2)
		command := strings.ToUpper(parts[0])
		args := ""
		if len(parts) > 1 {
			args = strings.TrimSpace(parts[1])
		}
		
		// Check if this line ends with a continuation
		if strings.HasSuffix(args, "\\") {
			args = strings.TrimSuffix(args, "\\")
			currentInstruction = &Instruction{
				Command: command,
				Args:    args,
			}
		} else {
			// Complete instruction
			dockerfile.Instructions = append(dockerfile.Instructions, Instruction{
				Command: command,
				Args:    args,
			})
		}
	}
	
	// Add any pending instruction
	if currentInstruction != nil {
		dockerfile.Instructions = append(dockerfile.Instructions, *currentInstruction)
	}
	
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading Dockerfile: %w", err)
	}
	
	return dockerfile, nil
}

// Execute processes the Dockerfile instructions
func (df *Dockerfile) Execute() error {
	fmt.Println("Executing Dockerfile instructions:")
	
	for _, instruction := range df.Instructions {
		fmt.Printf("  %s %s\n", instruction.Command, instruction.Args)
		
		// In a real implementation, we would:
		// 1. Create a new layer for each instruction
		// 2. Execute the instruction in a container
		// 3. Commit the changes to the layer
		
		// Here we'll just print what we would do
		switch instruction.Command {
		case "FROM":
			fmt.Printf("    (Would pull base image: %s)\n", instruction.Args)
		case "RUN":
			fmt.Printf("    (Would execute command: %s)\n", instruction.Args)
		case "COPY", "ADD":
			fmt.Printf("    (Would copy files: %s)\n", instruction.Args)
		case "CMD", "ENTRYPOINT":
			fmt.Printf("    (Would set default command: %s)\n", instruction.Args)
		case "ENV":
			fmt.Printf("    (Would set environment variable: %s)\n", instruction.Args)
		case "WORKDIR":
			fmt.Printf("    (Would set working directory: %s)\n", instruction.Args)
		case "EXPOSE":
			fmt.Printf("    (Would expose port: %s)\n", instruction.Args)
		default:
			fmt.Printf("    (Unknown instruction: %s)\n", instruction.Command)
		}
	}
	
	return nil
}