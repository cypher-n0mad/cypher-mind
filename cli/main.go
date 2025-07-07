// Go CLI for CypherMind orchestrator using Cobra and UNIX socket

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var (
	socketPath string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "mind [flags] <prompt>",
		Short: "Chat with your local CypherMind orchestrator",
		Args:  cobra.MinimumNArgs(1),
		Run:   run,
	}

	// Persistent flags
	home, _ := os.UserHomeDir()
	defaultSock := filepath.Join(home, ".local/run/ai.sock")
	rootCmd.PersistentFlags().StringVarP(&socketPath, "socket", "s", defaultSock,
		"UNIX socket path for AI orchestrator (overrides AI_SOCK env)")

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) {
	prompt := ""
	// Join all args into a single prompt
	if len(args) > 0 {
		prompt = cmd.Flags().Arg(0)
		if len(args) > 1 {
			prompt = ""
			for _, a := range args {
				prompt += a + " "
			}
			prompt = prompt[:len(prompt)-1]
		}
	}

	// Allow env override
	if env := os.Getenv("AI_SOCK"); env != "" {
		socketPath = env
	}

	// Check socket exists
	if _, err := os.Stat(socketPath); err != nil {
		log.Fatalf("Socket not found at %s: %v", socketPath, err)
	}

	// Build JSON payload
	payload := map[string]interface{}{"messages": []map[string]string{{"role": "user", "content": prompt}}}
	data, err := json.Marshal(payload)
	if err != nil {
		log.Fatalf("Failed to marshal payload: %v", err)
	}

	// HTTP client over UNIX socket
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", socketPath)
			},
		},
	}

	// Create request
	req, err := http.NewRequest("POST", "http://localhost/v1/chat/completions", bytes.NewReader(data))
	if err != nil {
		log.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Fatalf("Error response: %s", body)
	}

	// Stream response
	if _, err := io.Copy(os.Stdout, resp.Body); err != nil {
		log.Fatalf("Failed to read response: %v", err)
	}
}
