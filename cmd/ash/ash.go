package main

import (
	"ash/internal/config"
	"ash/internal/logger"
	sshlib "ash/internal/services/ssh"
	"ash/internal/services/terminal"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	cfg, args, err := config.Load()
	if err != nil {
		fmt.Printf("failed to load configuration file: %v\n", err)
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

	log.Info("starting ssh session")
	client, session, err := sshlib.StartSession(host)
	if err != nil {
		log.Error("failed to start ssh session", slog.Any("error", err))
		return
	}
	defer func() {
		err = session.Close()
		if err != nil {
			log.Warn("failed to close ssh session", slog.Any("error", err))
		}
		err := client.Close()
		if err != nil {
			log.Warn("failed to close ssh client", slog.Any("error", err))
		}
	}()

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var termErr error

	// Execute ssh.
	go func() {
		defer cancel()
		termErr = terminal.Associate(ctx, log, session)
		if termErr != nil {
			log.Error("error executing ssh")
		}
	}()

	// Listen for signals to perform graceful shutdown.
	sigs := make(chan os.Signal)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigs:
	case <-ctx.Done():
	}

	if termErr != nil {
		return
	}

	// Update host data.
	err = config.Save(cfg.Hosts)
	if err != nil {
		log.Error("error while saving config data", slog.Any("error", err))
		return
	}
}
