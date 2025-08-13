// cmd/chat.go
package cmd

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Messages []chatMessage `json:"messages"`
	Model    string        `json:"model,omitempty"`
}

var (
	sysPrompt string
	savePath  string
	modelName string
)

// chatCmd provides an interactive terminal chat that mirrors the prompt command's
// plain-text streaming behavior while keeping conversation history and /save, /clear, /exit.
var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Interactive chat session in your terminal",
	Long:  "Start an interactive chat session with the CypherMind orchestrator over the UNIX socket.",
	RunE:  runChat,
}

func init() {
	rootCmd.AddCommand(chatCmd)
	chatCmd.Flags().StringVar(&sysPrompt, "sys", "", "Optional system prompt to start the conversation")
	chatCmd.Flags().StringVar(&savePath, "save", "", "Save transcript to this file on exit or /save")
	chatCmd.Flags().StringVar(&modelName, "model", "", "Optional model name (if your server supports it)")
	chatCmd.Flags().StringVar(&socketPath, "sock", socketPath, "Path to UNIX domain socket (overrides AI_SOCK)")
}

func runChat(cmd *cobra.Command, _ []string) error {
	// Allow env override
	if env := os.Getenv("AI_SOCK"); env != "" {
		socketPath = env
	}
	if socketPath == "" {
		return fmt.Errorf("no socket path provided; use --sock or set AI_SOCK")
	}
	if _, err := os.Stat(socketPath); err != nil {
		return fmt.Errorf("socket not found at %s: %w", socketPath, err)
	}

	// HTTP client over UNIX socket (same pattern as prompt)
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", socketPath)
			},
			ForceAttemptHTTP2: false, // simpler streaming over HTTP/1.1
		},
		Timeout: 0, // streaming; no fixed timeout
	}

	// Conversation state
	var messages []chatMessage
	if sysPrompt != "" {
		messages = append(messages, chatMessage{Role: "system", Content: sysPrompt})
	}

	sc := bufio.NewScanner(os.Stdin)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	fmt.Println("Type your message and press Enter. Commands: /exit, /save [path], /clear")
	if savePath != "" {
		fmt.Printf("(Transcript will be saved to %s on exit)\n", savePath)
	}

	for {
		fmt.Print("\nyou> ")
		if !sc.Scan() {
			break
		}
		line := strings.TrimSpace(sc.Text())

		// Commands
		switch {
		case line == "/exit":
			return saveIfNeeded(messages)
		case line == "/clear":
			messages = nil
			if sysPrompt != "" {
				messages = append(messages, chatMessage{Role: "system", Content: sysPrompt})
			}
			fmt.Println("(conversation cleared)")
			continue
		case strings.HasPrefix(line, "/save"):
			parts := strings.Fields(line)
			if len(parts) > 1 {
				savePath = parts[1]
			}
			if err := writeTranscript(messages, savePath); err != nil {
				fmt.Printf("Save failed: %v\n", err)
			} else if savePath != "" {
				fmt.Printf("Saved to %s\n", savePath)
			} else {
				fmt.Println("No save path set. Use /save /path/to/file.json")
			}
			continue
		}

		if line == "" {
			continue
		}

		// Append user turn
		messages = append(messages, chatMessage{Role: "user", Content: line})

		// Prepare request body (no "stream" field; server returns plain text)
		reqBody := chatRequest{
			Messages: messages,
		}
		if modelName != "" {
			reqBody.Model = modelName
		}
		data, err := json.Marshal(reqBody)
		if err != nil {
			fmt.Printf("Marshal error: %v\n", err)
			continue
		}

		// Create request (same endpoint as prompt)
		req, err := http.NewRequest("POST", "http://localhost/v1/chat/completions", bytes.NewReader(data))
		if err != nil {
			fmt.Printf("Request error: %v\n", err)
			continue
		}
		req.Header.Set("Content-Type", "application/json")

		// Optional: timeout per request so we don't hang forever
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		req = req.WithContext(ctx)

		// Send request
		resp, err := client.Do(req)
		cancel()
		if err != nil {
			fmt.Printf("Request failed: %v\n", err)
			continue
		}
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			fmt.Printf("Error: HTTP %d\n%s\n", resp.StatusCode, strings.TrimSpace(string(body)))
			continue
		}

		// Stream raw text response to stdout, while buffering to append to history
		fmt.Print("ai > ")
		assistantText, err := readRawTextStream(resp.Body, os.Stdout)
		resp.Body.Close()
		if err != nil {
			fmt.Printf("\nread error: %v\n", err)
			continue
		}

		// Append assistant turn
		messages = append(messages, chatMessage{Role: "assistant", Content: assistantText})
	}

	if err := sc.Err(); err != nil {
		log.Printf("Input error: %v", err)
	}
	return saveIfNeeded(messages)
}

// readRawTextStream copies raw bytes to out and also buffers them to return a string.
func readRawTextStream(r io.Reader, out io.Writer) (string, error) {
	var buf bytes.Buffer
	_, err := io.Copy(io.MultiWriter(out, &buf), r)
	if err != nil {
		return "", err
	}
	s := buf.String()
	if !strings.HasSuffix(s, "\n") {
		fmt.Fprintln(out) // tidy newline
	}
	return s, nil
}

func saveIfNeeded(msgs []chatMessage) error {
	if savePath == "" {
		return nil
	}
	return writeTranscript(msgs, savePath)
}

func writeTranscript(msgs []chatMessage, path string) error {
	if path == "" {
		return errors.New("no save path provided")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	type transcript struct {
		Timestamp string        `json:"timestamp"`
		Messages  []chatMessage `json:"messages"`
	}
	out := transcript{
		Timestamp: time.Now().Format(time.RFC3339),
		Messages:  msgs,
	}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}
