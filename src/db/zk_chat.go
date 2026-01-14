/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package db

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

func buildZKChatPrompt(notes []ZKChatNote, message string) string {
	var sb strings.Builder

	sb.WriteString("Use the following zettelkasten notes as context. Each note is raw org-mode text.\n\n")

	for _, note := range notes {
		sb.WriteString(fmt.Sprintf("Note Title: %s\n", note.Title))
		sb.WriteString(fmt.Sprintf("Note ID: %s\n", note.ID))
		sb.WriteString("Note Content:\n")
		sb.WriteString(note.Content)
		if !strings.HasSuffix(note.Content, "\n") {
			sb.WriteString("\n")
		}
		sb.WriteString("\n---\n\n")
	}

	sb.WriteString("User question:\n")
	sb.WriteString(message)

	return sb.String()
}

// StreamZKChat streams a zettelkasten chat response from Ollama.
func StreamZKChat(ctx context.Context, notes []ZKChatNote, message string, onChunk func(string) error) error {
	config, err := GetOllamaConfig()
	if err != nil {
		return err
	}

	prompt := buildZKChatPrompt(notes, message)

	systemPrompt := "You are a research assistant for a personal zettelkasten. Use the provided notes as primary sources. If details are missing, say so clearly. Cite note titles when referencing information. Provide structured, concise responses. Use markdown for emphasis and lists, but avoid headings."

	reqBody := chatRequest{
		Model:  config.Model,
		Stream: true,
		Messages: []chatMessage{
			{
				Role:    "system",
				Content: systemPrompt,
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	endpoint := strings.TrimSuffix(config.URL, "/") + "/v1/chat/completions"
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 300 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to call Ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Ollama returned status %d: %s", resp.StatusCode, string(body))
	}

	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("failed to read stream: %w", err)
		}

		lineStr := strings.TrimSpace(string(line))
		if lineStr == "" {
			continue
		}

		if !strings.HasPrefix(lineStr, "data: ") {
			continue
		}

		data := strings.TrimPrefix(lineStr, "data: ")
		if data == "[DONE]" {
			break
		}

		var chatResp chatResponse
		if err := json.Unmarshal([]byte(data), &chatResp); err != nil {
			continue
		}

		if chatResp.Error != nil {
			return fmt.Errorf("Ollama error: %s", chatResp.Error.Message)
		}

		if len(chatResp.Choices) > 0 {
			content := chatResp.Choices[0].Delta.Content
			if content != "" {
				if err := onChunk(content); err != nil {
					return err
				}
			}
		}
	}

	return nil
}
