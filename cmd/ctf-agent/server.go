package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/Conly-Zy/CTF-Agent/internal/api"
	"github.com/Conly-Zy/CTF-Agent/internal/config"
	"github.com/Conly-Zy/CTF-Agent/internal/store"
	"github.com/spf13/cobra"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the web server",
	Long:  "Start the HTTP API server with web interface",
	RunE:  runServer,
}

var (
	serverAddr   string
	serverConfig string
	dbPath       string
)

func init() {
	serverCmd.Flags().StringVar(&serverAddr, "addr", ":8080", "Server address")
	serverCmd.Flags().StringVarP(&serverConfig, "config", "c", "", "Config file path")
	serverCmd.Flags().StringVar(&dbPath, "db", "ctf-agent.db", "Database path")

	rootCmd.AddCommand(serverCmd)
}

func runServer(cmd *cobra.Command, args []string) error {
	_, err := config.Load(serverConfig)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	db, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	server := api.NewServer(db, logger, serverAddr)

	fmt.Printf("Starting CTF-Agent web server...\n")
	fmt.Printf("Address: %s\n", serverAddr)
	fmt.Printf("Database: %s\n", dbPath)

	return server.Start()
}
