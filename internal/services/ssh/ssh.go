package ssh

import (
	"ash/internal/config"
	"fmt"
	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
	"os"
	"strings"
)

func StartSession(host *config.SSHHost) (*ssh.Client, *ssh.Session, error) {
	authMethods, err := getAuthMethods(host)
	if err != nil {
		return nil, nil, err
	}

	sshConfig := &ssh.ClientConfig{
		User:            host.User,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // ⚠️ не использовать в проде
	}

	// Create new SSH session.
	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", host.HostName, host.Port), sshConfig)
	if err != nil {
		return nil, nil, err
	}

	session, err := client.NewSession()
	if err != nil {
		return nil, nil, err
	}

	return client, session, nil
}

func getAuthMethods(host *config.SSHHost) ([]ssh.AuthMethod, error) {
	var authMethods []ssh.AuthMethod

	if host.Password != "" {
		authMethods = append(authMethods, ssh.Password(host.Password))
	} else if host.IdentityFile != "" {
		// Try to parse ssh key.
		key, err := os.ReadFile(host.IdentityFile)
		if err != nil {
			return nil, err
		}
		signer, err := ssh.ParsePrivateKey(key)

		if err != nil {
			if !strings.Contains(err.Error(), "ssh: this private key is passphrase protected") {
				return nil, err
			}

			// Key is passphrase protected. Prompt with password.
			fmt.Printf("Enter passphrase for %s: ", host.IdentityFile)
			passphrase, err := term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Println()
			if err != nil {
				fmt.Println("Failed to read passphrase")
				return nil, err
			}

			// Try to parse ssh key with passphrase.
			signer, err = ssh.ParsePrivateKeyWithPassphrase(key, passphrase)
			if err != nil {
				fmt.Println("Wrong passphrase")
				return nil, fmt.Errorf("failed to decrypt SSH key: %w", err)
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

	return authMethods, nil
}
