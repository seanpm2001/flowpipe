package container

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/turbot/flowpipe-functions/docker"
)

type Container struct {

	// Configuration
	Name  string            `json:"name"`
	Image string            `json:"image"`
	Cmd   []string          `json:"cmd"`
	Env   map[string]string `json:"env"`

	// Runtime information
	CreatedAt   *time.Time               `json:"created_at,omitempty"`
	UpdatedAt   *time.Time               `json:"updated_at,omitempty"`
	ImageExists bool                     `json:"image_exists"`
	Runs        map[string]*ContainerRun `json:"runs"`

	// Internal
	ctx          context.Context
	dockerClient *docker.DockerClient
}

type ContainerRun struct {
	ContainerID string `json:"container_id"`
	Status      string `json:"status"`
	Output      string `json:"output"`
}

// Option defines a function signature for configuring the Docker client.
type ContainerOption func(*Container) error

// WithContext configures the Docker client with a specific context.
func WithContext(ctx context.Context) ContainerOption {
	return func(c *Container) error {
		c.ctx = ctx
		return nil
	}
}

// WithConfigDockerClient configures the Docker client.
func WithDockerClient(client *docker.DockerClient) ContainerOption {
	return func(c *Container) error {
		c.dockerClient = client
		return nil
	}
}

// NewConfig creates a new Function Config with the provided options.
func NewContainer(options ...ContainerOption) (*Container, error) {

	now := time.Now()

	fc := &Container{
		CreatedAt: &now,
		Cmd:       []string{},
		Env:       map[string]string{},
		Runs:      map[string]*ContainerRun{},
		//ImageExists: true,
	}

	for _, option := range options {
		if err := option(fc); err != nil {
			return nil, err
		}
	}

	if fc.ctx == nil {
		fc.ctx = context.Background()
	}

	return fc, nil
}

func (c *Container) GetEnv() []string {
	env := []string{}
	for k, v := range c.Env {
		// TODO - escape me
		//env = append(env, fmt.Sprintf("%s=%s", strconv.Quote(k), strconv.Quote(v)))
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	return env
}

// SetUpdatedAt sets the updated at time.
func (c *Container) SetUpdatedAt() {
	now := time.Now()
	c.UpdatedAt = &now
}

// Load loads the function config.
func (c *Container) Load() error {
	if err := c.Validate(); err != nil {
		return err
	}
	return nil
}

// Unload unloads the function config.
func (c *Container) Unload() error {
	// Cleanup artifacts from Docker
	return c.CleanupArtifacts()
}

// Validate validates the function config.
func (c *Container) Validate() error {

	if c.Name == "" {
		return fmt.Errorf("name required for container")
	}

	if c.Image == "" {
		return fmt.Errorf("image required for container: %s", c.Name)
	}

	return nil
}

func (c *Container) SetRunStatus(containerID string, newStatus string) error {
	if c.Runs[containerID] == nil {
		c.Runs[containerID] = &ContainerRun{
			ContainerID: containerID,
		}
	}
	c.Runs[containerID].Status = newStatus
	return nil
}

func (c *Container) Run() (string, error) {

	containerID := ""

	start := time.Now()

	// Pull the Docker image if it's not already available
	if !c.ImageExists {
		imageExistsStart := time.Now()
		imageExists, err := c.dockerClient.ImageExists(c.Image)
		fmt.Printf("imageExists: %s\n", time.Since(imageExistsStart))

		if err != nil {
			return containerID, err
		}
		if !imageExists {
			imagePullStart := time.Now()
			err = c.dockerClient.ImagePull(c.Image)
			fmt.Printf("imagePull elapsed: %s\n", time.Since(imagePullStart))
			if err != nil {
				return containerID, err
			}
			// Image has been pulled, so now exists locally
			c.ImageExists = true
		}
	}

	// Enforce a timeout to prevent runaway containers
	// TODO - should be a container config option
	timeout := 60

	// Create a container using the specified image
	createConfig := container.Config{
		Image: c.Image,
		Cmd:   c.Cmd,
		Labels: map[string]string{
			// Set on the container since it's not on the image
			"io.flowpipe.type": "container",
			"io.flowpipe.name": c.Name,
			// Is this standard for containers?
			"org.opencontainers.container.created": time.Now().Format(time.RFC3339),
		},
		Env:         c.GetEnv(),
		StopTimeout: &timeout,

		// TODO - FIX ME
		// I'm confused about how this works, and we're seeing a lot of control characters
		// with AWS CLI output.
		//
		// Tty: true, --no-cli-pager
		// Works! Format is good. But it's complicated and unexpected to need these.
		//
		// Tty: false, --no-cli-pager
		// Control chars in output.
		//
		// Tty: false
		// Control chars in output.
		//
		// Tty: true
		// Hangs, I presume because it's waiting for input on the paging.
		//
		// Overall, I trust this StackOverflow answer which says we want to use
		// docker run (without -it) to avoid control chars. But, we're still getting
		// the chars for some reason.
		// https://stackoverflow.com/questions/65824304/aws-cli-returns-json-with-control-codes-making-jq-fail
		//
		// The control characters seem to be at the start of each line:
		// [1 0 0 0 0 0 0 2 123 10 1 0 0 0 0 0 0 14 32 32 32 32 34 86 112 99 115 34 58 32 91 10
		// Specifically, see the "1 0 0 0 0 0 0 <number>" at the start of each line, which is:
		// 1 0 0 0 0 0 0  2 {
		// 1 0 0 0 0 0 0 14     "Vpcs": [
		//
		// OK - here is the answer - https://github.com/moby/moby/issues/7375#issuecomment-51462963
		// Docker adds a control character to each line of output to indicate if
		// it's stdout or stderr.
		Tty:          false, // Turn off interactive mode
		OpenStdin:    false, // Turn off stdin
		AttachStdin:  false,
		AttachStdout: false,
		AttachStderr: false,
	}
	containerCreateStart := time.Now()
	containerResp, err := c.dockerClient.CLI.ContainerCreate(c.ctx, &createConfig, &container.HostConfig{}, &network.NetworkingConfig{}, nil, "")
	fmt.Printf("containerCreate elapsed: %s\n", time.Since(containerCreateStart))
	if err != nil {
		return containerID, err
	}
	containerID = containerResp.ID
	c.SetRunStatus(containerID, "created")

	// Start the container
	containerStartStart := time.Now()
	err = c.dockerClient.CLI.ContainerStart(c.ctx, containerID, types.ContainerStartOptions{})
	fmt.Printf("containerStart elapsed: %s\n", time.Since(containerStartStart))
	if err != nil {
		return containerID, err
	}
	c.SetRunStatus(containerID, "started")

	// Wait for the container to finish
	containerWaitStart := time.Now()
	statusCh, errCh := c.dockerClient.CLI.ContainerWait(c.ctx, containerID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return containerID, err
		}
	case <-statusCh:
	}
	fmt.Printf("containerWait elapsed: %s\n", time.Since(containerWaitStart))

	c.SetRunStatus(containerID, "finished")

	// Retrieve the container output
	outputBuf := new(bytes.Buffer)
	containerLogsOptions := types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		// Timstamps inject timestamp text into the output, making it hard to parse
		Timestamps: false,
		// Get all logs from the container, not just the last X lines
		Tail: "all",
	}
	containerLogsStart := time.Now()
	reader, err := c.dockerClient.CLI.ContainerLogs(c.ctx, containerID, containerLogsOptions)
	fmt.Printf("containerLogs elapsed: %s\n", time.Since(containerLogsStart))
	if err != nil {
		return containerID, err
	}
	defer reader.Close()

	o := NewOutput()
	err = o.FromDockerLogsReader(reader)
	if err != nil {
		return containerID, err
	}
	c.Runs[containerID].Output = o.Combined()

	/*
		stdoutStr, stderrStr, err := processLogs(reader)
		if err != nil {
			return containerID, err
		}
		c.Runs[containerID].Output = stdoutStr + stderrStr
	*/

	/*
		_, err = outputBuf.ReadFrom(reader)
		if err != nil {
			return containerID, err
		}
	*/

	/*
		encoding := charmap.ISO8859_1 // Example: ISO 8859-1 (Latin-1)
		// Convert the byte buffer to a string using the specified encoding
		decoder := encoding.NewDecoder()
		output, _ := decoder.Bytes(outputBuf.Bytes())
		c.Runs[containerID].Output = string(output)
	*/

	//c.Runs[containerID].Output = outputBuf.String()

	fmt.Printf("%v", outputBuf.Bytes())
	fmt.Printf("%s", c.Runs[containerID].Output)

	c.SetRunStatus(containerID, "logged")

	// Remove the container
	containerRemoveStart := time.Now()
	err = c.dockerClient.CLI.ContainerRemove(c.ctx, containerID, types.ContainerRemoveOptions{})
	fmt.Printf("containerRemove elapsed: %s\n", time.Since(containerRemoveStart))
	if err != nil {
		// TODO - do we have to fail here? Perhaps things like not found can be ignored?
		return containerID, err
	}

	c.SetRunStatus(containerID, "removed")

	fmt.Printf("container [%s]: %s\n", containerID, time.Since(start))

	return containerID, nil
}

// Cleanup all docker containers for the given container.
func (c *Container) CleanupArtifacts() error {
	return c.dockerClient.CleanupArtifactsForLabel("io.flowpipe.name", c.Name)
}

func processLogs(reader io.Reader) (string, string, error) {
	header := make([]byte, 8)
	stdout := bytes.Buffer{}
	stderr := bytes.Buffer{}

	for {
		_, err := reader.Read(header)
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", "", err
		}

		streamType := header[0]
		payloadSize := binary.BigEndian.Uint32(header[4:8])
		payload := make([]byte, payloadSize)

		_, err = io.ReadFull(reader, payload)
		if err != nil {
			return "", "", err
		}

		switch streamType {
		case 2:
			stderr.Write(payload)
		default:
			stdout.Write(payload)
		}
	}

	// Process stdout and stderr as needed
	fmt.Println("Stdout:", stdout.String())
	fmt.Println("Stderr:", stderr.String())

	return stdout.String(), stderr.String(), nil
}
