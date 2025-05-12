package main

import (
	"ash/internal/config"
	"ash/internal/logger"
	"context"
	"fmt"
	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

func main() {
	cfg, args, err := config.Load()
	if err != nil {
		fmt.Println(err)
		return
	}

	log := logger.New(cfg.Verbose)

	// Find host related to alias.
	var host *config.SSHHost
	for _, h := range cfg.Hosts {
		if h.Alias == args.Alias {
			host = h
		}
	}
	if host == nil {
		if args.User == nil || args.Host == nil {
			log.Error("specified alias does not exist")
			return
		}

		host = &config.SSHHost{
			Alias:        args.Alias,
			User:         *args.User,
			HostName:     *args.Host,
			Port:         args.Port,
			IdentityFile: args.IdentityFile,
		}
		cfg.Hosts = append(cfg.Hosts, host)
	}

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var sshErr error

	// Execute ssh.
	go func() {
		defer cancel()
		sshErr = execute(ctx, host)
		if sshErr != nil {
			log.Error("error while executing running ssh", slog.Any("error", sshErr))
		}
	}()

	// Listen for signals to perform graceful shutdown.
	sigs := make(chan os.Signal)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigs:
	case <-ctx.Done():
	}

	if sshErr != nil {
		return
	}

	// Update host data.
	err = config.Save(cfg.Hosts)
	if err != nil {
		log.Error("error while saving config data", slog.Any("error", err))
		return
	}

	fmt.Println("Bye!")
}

func execute(ctx context.Context, host *config.SSHHost) error {
	var authMethods []ssh.AuthMethod

	if host.Password != "" {
		authMethods = append(authMethods, ssh.Password(host.Password))
	} else if host.IdentityFile != "" {
		// Try to parse ssh key.
		key, err := os.ReadFile(host.IdentityFile)
		if err != nil {
			return err
		}
		signer, err := ssh.ParsePrivateKey(key)

		if err != nil {
			if !strings.Contains(err.Error(), "ssh: this private key is passphrase protected") {
				return err
			}

			// Key is passphrase protected. Prompt with password.
			fmt.Printf("Enter passphrase for %s: ", host.IdentityFile)
			passphrase, err := term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Println()
			if err != nil {
				fmt.Println("Failed to read passphrase")
				return err
			}

			// Try to parse ssh key with passphrase.
			signer, err = ssh.ParsePrivateKeyWithPassphrase(key, passphrase)
			if err != nil {
				fmt.Println("Wrong passphrase")
				return fmt.Errorf("failed to decrypt SSH key: %w", err)
			}
		}

		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}

	promptMethod := ssh.PasswordCallback(func() (string, error) {
		fmt.Print("Password: ")
		bytePassword, err := term.ReadPassword(int(os.Stdin.Fd()))
		host.Password = string(bytePassword)
		fmt.Println()
		return string(bytePassword), err
	})

	authMethods = append(authMethods, promptMethod)

	sshConfig := &ssh.ClientConfig{
		User:            host.User,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // ⚠️ не использовать в проде
	}

	// Create new SSH session.
	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", host.HostName, host.Port), sshConfig)
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer func() { _ = session.Close() }()

	// Configure interactive terminal.
	session.Stdout = os.Stdout
	session.Stderr = os.Stderr
	session.Stdin = os.Stdin

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := session.RequestPty("xterm", 80, 160, modes); err != nil {
		return err
	}

	// Switch terminal to raw mode.
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("failed to set terminal to raw mode: %w", err)
	}
	defer func() {
		_ = term.Restore(int(os.Stdin.Fd()), oldState)
	}()

	// Start interactive shell.
	if err := session.Shell(); err != nil {
		return err
	}

	sessionChan := make(chan error)

	// Wait for finish.
	go func() {
		err = session.Wait()
		if err != nil && err != io.EOF {
			sessionChan <- err
		}
		sessionChan <- nil
	}()

	select {
	case <-ctx.Done():
		return nil
	case err := <-sessionChan:
		return err
	}
}
