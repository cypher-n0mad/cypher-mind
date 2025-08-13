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
	Messages []chatMessage       `json:"messages"`
	Stream   bool                `json:"stream"`
	Model    string              `json:"model,omitempty"`
	Extra    map[string]any      `json:"-"`
}

type streamChoiceDelta struct {
	Content string `json:"content"`
	Role    string `json:"role,omitempty"`
}

type streamChoice struct {
	Delta        streamChoiceDelta `json:"delta"`
	FinishReason string            `json:"finish_reason"`
	Index        int               `json:"index"`
}

type streamChunk struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Choices []streamChoice `json:"choices"`
}

type oneShotChoice struct {
	Index int `json:"index"`
	// OpenAI style
	Message *chatMessage `json:"message,omitempty"`
	// Some servers return "delta" even when not streaming — be tolerant
	Delta *streamChoiceDelta `json:"delta,omitempty"`
}

type oneShotResponse struct {
	Choices []oneShotChoice `json:"choices"`
}

var (
	sysPrompt   string
	savePath    string
	noStream    bool
	modelName   string
)

// chatCmd provides an interactive terminal chat.
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
	chatCmd.Flags().BoolVar(&noStream, "no-stream", false, "Disable streaming; expect one-shot JSON response")
	chatCmd.Flags().StringVar(&modelName, "model", "", "Optional model name (if your server supports it)")
	// Optional: also allow overriding socket by flag (env AI_SOCK still works)
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

	// Build HTTP client over UNIX socket
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", socketPath)
			},
			ForceAttemptHTTP2: false, // over UDS, stick to HTTP/1.1 for simpler streaming
		},
		Timeout: 0, // streaming; no fixed timeout
	}

	// Conversation state
	var messages []chatMessage
	if sysPrompt != "" {
		messages = append(messages, chatMessage{Role: "system", Content: sysPrompt})
	}

	sc := bufio.NewScanner(os.Stdin)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024) // allow large inputs

	fmt.Println("Type your message and press Enter. Commands: /exit, /save, /clear")
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
			} else {
				fmt.Printf("Saved to %s\n", savePath)
			}
			continue
		}

		if line == "" {
			continue
		}

		// Append user turn
		messages = append(messages, chatMessage{Role: "user", Content: line})

		// Prepare request
		reqBody := chatRequest{
			Messages: messages,
			Stream:   !noStream,
		}
		if modelName != "" {
			reqBody.Model = modelName
		}

		data, err := json.Marshal(reqBody)
		if err != nil {
			fmt.Printf("Marshal error: %v\n", err)
			continue
		}

		req, err := http.NewRequest("POST", "http://localhost/v1/chat/completions", bytes.NewReader(data))
		if err != nil {
			fmt.Printf("Request error: %v\n", err)
			continue
		}
		req.Header.Set("Content-Type", "application/json")

		// Do request
		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("Request failed: %v\n", err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			fmt.Printf("Error: %s\n", body)
			continue
		}

		// Read response (streaming or non-streaming)
		fmt.Print("ai > ")
		var assistantText string
		if !noStream {
			assistantText, err = readStream(resp.Body, os.Stdout)
		} else {
			assistantText, err = readOneShot(resp.Body)
		}
		resp.Body.Close()
		if err != nil {
			fmt.Printf("\nParse error: %v\n", err)
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

// readStream parses an OpenAI-style SSE stream and writes tokens to out.
// It returns the concatenated assistant message.
func readStream(r io.Reader, out io.Writer) (string, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	var builder strings.Builder

	for sc.Scan() {
		line := sc.Text()
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Expect lines like: "data: {json}" or "data: [DONE]"
		if !strings.HasPrefix(line, "data:") {
			// Some servers send raw JSON lines without the "data:" prefix.
			// Try to parse anyway.
			if s := strings.TrimSpace(line); s == "[DONE]" {
				break
			}
			if tok, ok := tryExtractDelta(line); ok {
				builder.WriteString(tok)
				fmt.Fprint(out, tok)
				continue
			}
			continue
		}

		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			if payload == "[DONE]" {
				break
			}
			continue
		}

		if tok, ok := tryExtractDelta(payload); ok {
			builder.WriteString(tok)
			fmt.Fprint(out, tok)
		}
	}
	// Print trailing newline for neatness
	fmt.Fprintln(out)
	return builder.String(), sc.Err()
}

// tryExtractDelta tries to parse a stream chunk and return the delta content.
func tryExtractDelta(s string) (string, bool) {
	var chunk streamChunk
	if err := json.Unmarshal([]byte(s), &chunk); err != nil {
		return "", false
	}
	if len(chunk.Choices) == 0 {
		return "", false
	}
	return chunk.Choices[0].Delta.Content, true
}

// readOneShot handles a non-streaming JSON response and returns the assistant message.
func readOneShot(r io.Reader) (string, error) {
	body, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	var resp oneShotResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", errors.New("no choices in response")
	}
	if resp.Choices[0].Message != nil {
		return resp.Choices[0].Message.Content, nil
	}
	if resp.Choices[0].Delta != nil {
		return resp.Choices[0].Delta.Content, nil
	}
	// Fallback: print raw
	return string(body), nil
}
