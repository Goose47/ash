package config

import (
	"ash/internal/util/files"
	"errors"
	"flag"
	"fmt"
	"github.com/ilyakaznacheev/cleanenv"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	Hosts   []*SSHHost `yaml:"hosts,omitempty"`
	Verbose bool       `yaml:"verbose,omitempty"`
}

type SSHHost struct {
	Alias        string `yaml:"alias"`
	HostName     string `yaml:"host_name"`
	User         string `yaml:"user"`
	Password     string `yaml:"password"`
	Port         int    `yaml:"port"`
	IdentityFile string `yaml:"identityFile"`
}

type Args struct {
	User         *string
	Host         *string
	Alias        string
	Port         int
	IdentityFile string
}

var configPath string

func Load() (*Config, *Args, error) {
	// Define flags.
	var port int
	var identityFile string
	var verbose bool

	identityFilePath, err := getDefaultIdentityFilePath()
	if err != nil {
		return nil, nil, fmt.Errorf("%w", err)
	}

	flag.StringVar(&configPath, "config", "", "path to config file")
	flag.IntVar(&port, "p", 22, "port")
	flag.StringVar(&identityFile, "i", identityFilePath, "identity file")
	flag.BoolVar(&verbose, "v", false, "verbose")

	flag.Parse()

	// Fetch default config path.
	if configPath == "" {
		var err error
		configPath, err = fetchConfigPath()
		if err != nil {
			return nil, nil, fmt.Errorf("%w", err)
		}
	}

	// Parse config file.
	var cfg Config
	err = cleanenv.ReadConfig(configPath, &cfg)
	if err != nil && !strings.Contains(err.Error(), "config file parsing error: EOF") {
		return nil, nil, fmt.Errorf("%w", err)
	}

	// Add cli arguments to config.
	var sshArg string
	var user *string
	var host *string
	var alias string

	args := flag.Args()

	switch len(args) {
	case 0:
		return nil, nil, fmt.Errorf("%w", errors.New("too few arguments"))
	case 1:
		alias = args[0]
	case 2:
		sshArg, alias = args[0], args[1]
	default:
		return nil, nil, fmt.Errorf("%w", errors.New("too many arguments"))
	}

	if sshArg != "" {
		userAddr := strings.Split(sshArg, "@")
		if len(userAddr) != 2 {
			return nil, nil, fmt.Errorf("invalid ssh argument: expected user@host, got %s", sshArg)
		}
		user, host = &userAddr[0], &userAddr[1]
	}

	run := &Args{
		User:         user,
		Host:         host,
		Alias:        alias,
		Port:         port,
		IdentityFile: identityFile,
	}

	cfg.Verbose = verbose

	return &cfg, run, nil
}

func Save(hosts []*SSHHost) error {
	data, err := yaml.Marshal(Config{Hosts: hosts})
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	var path string
	if configPath != "" {
		path = configPath
	} else {
		path, err = fetchConfigPath()
		if err != nil {
			return fmt.Errorf("%w", err)
		}
	}

	err = os.WriteFile(path, data, 0644)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

func fetchConfigPath() (string, error) {
	// Get home directory.
	homePath, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("%w", err)
	}

	// Check if ash directory exists. Create if not.
	ashPath := filepath.Join(homePath, ".ash")
	ashPathExists, err := files.Exists(ashPath)
	if err != nil {
		return "", fmt.Errorf("%w", err)
	}
	if !ashPathExists {
		err := os.Mkdir(ashPath, os.ModePerm)
		if err != nil {
			return "", fmt.Errorf("%w", err)
		}
	}

	// Check if ash config file exists. Create if not.
	ashConfigPath := filepath.Join(ashPath, "config.yml")
	exists, err := files.Exists(ashConfigPath)
	if err != nil {
		return "", fmt.Errorf("%w", err)
	}
	if !exists {
		f, err := os.Create(ashConfigPath)
		if err != nil {
			return "", fmt.Errorf("%w", err)
		}
		_ = f.Close()
	}

	return ashConfigPath, nil
}

func getDefaultIdentityFilePath() (string, error) {
	// Get home directory.
	homePath, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("%w", err)
	}

	// Check if ssh directory exists.
	sshPath := filepath.Join(homePath, ".ssh")
	sshPathExists, err := files.Exists(sshPath)
	if err != nil {
		return "", fmt.Errorf("%w", err)
	}
	if !sshPathExists {
		return "", nil
	}

	// Check if default ssh key exists.
	sshKeyPath := filepath.Join(homePath, ".ssh", "id_rsa")
	sshKeyExists, err := files.Exists(sshKeyPath)
	if err != nil {
		return "", fmt.Errorf("%w", err)
	}
	if !sshKeyExists {
		return "", nil
	}
	return sshKeyPath, nil
}
