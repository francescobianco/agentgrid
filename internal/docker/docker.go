package docker

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

type Result struct {
	Output string
	Error  error
}

func RunAgent(prompt, dockerImage string) Result {
	cmd := exec.Command("docker", "run", "--rm",
		"-e", "PROMPT="+prompt,
		dockerImage,
		"sh", "-c", "echo \"$PROMPT\"",
	)

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
