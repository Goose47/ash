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

	client, session, err := sshlib.StartSession(host)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer func() {
		_ = client.Close()
		_ = session.Close()
	}()

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var termErr error

	// Execute ssh.
	go func() {
		defer cancel()
		termErr = terminal.Associate(ctx, session)
		if termErr != nil {
			log.Error("error while executing running ssh", slog.Any("error", termErr))
		}
	}()

	// Listen for signals to perform graceful shutdown.
	sigs := make(chan os.Signal)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigs:
		fmt.Println("signal received")
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

	fmt.Println("Bye!")
}
