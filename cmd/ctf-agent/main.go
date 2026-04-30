package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "ctf-agent",
	Short: "An AI-powered CTF solving agent",
	Long:  "CTF-Agent is an intelligent agent that automatically solves CTF challenges using Claude AI.",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func main() {
	Execute()
}
