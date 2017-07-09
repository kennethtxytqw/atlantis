package terraform

import (
	"fmt"
	"os/exec"
	"regexp"

	"strings"

	version "github.com/hashicorp/go-version"
	"github.com/hootsuite/atlantis/logging"
	"github.com/pkg/errors"
)

type Client struct {
	defaultVersion *version.Version
}

var versionRegex = regexp.MustCompile("Terraform v(.*)\n")

func NewClient() (*Client, error) {
	versionCmdOutput, err := exec.Command("terraform", "version").CombinedOutput()
	output := string(versionCmdOutput)
	if err != nil {
		// exec.go line 35, Error() returns
		// "exec: " + strconv.Quote(e.Name) + ": " + e.Err.Error()
		if err.Error() == fmt.Sprintf("exec: \"terraform\": %s", exec.ErrNotFound.Error()) {
			return nil, errors.New("terraform not found in $PATH. \n\nDownload terraform from https://www.terraform.io/downloads.html")
		}
		return nil, errors.Wrapf(err, "running terraform version: %s", output)
	}
	match := versionRegex.FindStringSubmatch(output)
	if len(match) <= 1 {
		return nil, fmt.Errorf("could not parse terraform version from %s", output)
	}
	version, err := version.NewVersion(match[1])
	if err != nil {
		return nil, errors.Wrap(err, "parsing terraform version")
	}

	return &Client{
		defaultVersion: version,
	}, nil
}

func (c *Client) RunCommand(log *logging.SimpleLogger, path string, tfCmd []string, tfEnvVars []string) (string, error) {
	return c.RunCommandWithVersion(log, path, tfCmd, tfEnvVars, c.defaultVersion)
}

func (c *Client) Version() *version.Version {
	return c.defaultVersion
}

func (c *Client) RunCommandWithVersion(log *logging.SimpleLogger, path string, tfCmd []string, tfEnvVars []string, v *version.Version) (string, error) {
	tfExecutable := "terraform"
	// if version is the same as the default, don't need to prepend the version name to the executable
	if !v.Equal(c.defaultVersion) {
		tfExecutable = fmt.Sprintf("%s%s", tfExecutable, v.String())
	}
	terraformCmd := exec.Command(tfExecutable, tfCmd...)
	terraformCmd.Dir = path
	if len(tfEnvVars) > 0 {
		terraformCmd.Env = tfEnvVars
	}
	out, err := terraformCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s: running %q in %q", err, strings.Join(terraformCmd.Args, " "), path)
	}
	log.Info("successfully ran %q in %q", strings.Join(terraformCmd.Args, " "), path)
	return string(out), nil
}

func (c *Client) RunInitAndEnv(log *logging.SimpleLogger, path string, env string, extraArgs []string) ([]string, error) {
	var outputs []string
	// run terraform init
	output, err := c.RunCommand(log, path, append([]string{"init", "-no-color"}, extraArgs...), nil)
	if err != nil {
		return nil, err
	}
	outputs = append(outputs, output)

	// run terraform env new and select
	output, err = c.RunCommand(log, path, []string{"env", "select", "-no-color", env}, nil)
	if err != nil {
		// if terraform env select fails we will run terraform env new
		// to create a new environment
		output, err = c.RunCommand(log, path, []string{"env", "new", "-no-color", env}, nil)
		if err != nil {
			return nil, err
		}
	}
	return append(outputs, output), nil
}
