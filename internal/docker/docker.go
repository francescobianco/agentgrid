package docker

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Result struct {
	Output string
	Error  error
}

func RunAgent(prompt, dockerImage, workingDir string) Result {
	args := []string{"run", "--rm"}
	if workingDir != "" {
		args = append(args, "-v", workingDir+":/work")
		args = append(args, "-w", "/work")
	}
	args = append(args, "-e", "PROMPT="+prompt, dockerImage, "sh", "-c", "echo \"$PROMPT\"")

	cmd := exec.Command("docker", args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := strings.TrimSpace(stdout.String())
	if stderr.Len() > 0 {
		output += "\nSTDERR: " + strings.TrimSpace(stderr.String())
	}

	return Result{Output: output, Error: err}
}

func BuildImage(dockerfile, tag string) Result {
	dir, err := os.MkdirTemp("", "agentgrid-build-*")
	if err != nil {
		return Result{Output: "", Error: fmt.Errorf("create temp dir: %w", err)}
	}
	defer os.RemoveAll(dir)

	if err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte(dockerfile), 0644); err != nil {
		return Result{Output: "", Error: fmt.Errorf("write Dockerfile: %w", err)}
	}

	cmd := exec.Command("docker", "build", "-t", tag, ".")
	cmd.Dir = dir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	output := strings.TrimSpace(stdout.String())
	if stderr.Len() > 0 {
		output += "\nSTDERR: " + strings.TrimSpace(stderr.String())
	}

	return Result{Output: output, Error: err}
}

func IsAvailable() error {
	cmd := exec.Command("docker", "info")
	return cmd.Run()
}

func PullImage(image string) error {
	cmd := exec.Command("docker", "pull", image)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pull image: %w\n%s", err, stderr.String())
	}
	return nil
}
