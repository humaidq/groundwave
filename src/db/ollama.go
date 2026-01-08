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
	"os"
	"strings"
	"time"
)

// OllamaConfig holds the Ollama server configuration
type OllamaConfig struct {
	URL   string
	Model string
}

// OpenAI-compatible request/response structures
type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Stream   bool          `json:"stream,omitempty"`
}

type chatChoice struct {
	Message chatMessage `json:"message"`
	Delta   chatMessage `json:"delta,omitempty"` // For streaming responses
}

type chatResponse struct {
	Choices []chatChoice `json:"choices"`
	Error   *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// GetOllamaConfig loads Ollama configuration from environment variables
func GetOllamaConfig() (*OllamaConfig, error) {
	url := os.Getenv("OLLAMA_URL")
	model := os.Getenv("OLLAMA_MODEL")

	if url == "" || model == "" {
		return nil, fmt.Errorf("Ollama configuration incomplete: OLLAMA_URL and OLLAMA_MODEL must be set")
	}

	return &OllamaConfig{
		URL:   url,
		Model: model,
	}, nil
}

// LabResultSummary holds lab result data for AI summarization
type LabResultSummary struct {
	TestName    string
	TestValue   float64
	TestUnit    string
	RefMin      *float64
	RefMax      *float64
	RangeStatus string // "normal", "out_of_reference", "out_of_optimal"
}

// buildLabSummaryPrompt creates the prompt for lab result summarization
func buildLabSummaryPrompt(profile *HealthProfile, followup *HealthFollowup, results []LabResultSummary) string {
	var sb strings.Builder

	sb.WriteString("Please summarize the following lab test results:\n\n")

	// Add patient info if available
	sb.WriteString(fmt.Sprintf("Patient: %s\n", profile.Name))
	if profile.DateOfBirth != nil {
		age := time.Now().Year() - profile.DateOfBirth.Year()
		sb.WriteString(fmt.Sprintf("Age: %d years\n", age))
	}
	if profile.Gender != nil {
		sb.WriteString(fmt.Sprintf("Gender: %s\n", *profile.Gender))
	}
	sb.WriteString(fmt.Sprintf("Visit Date: %s\n", followup.FollowupDate.Format("January 2, 2006")))
	sb.WriteString(fmt.Sprintf("Facility: %s\n", followup.HospitalName))
	sb.WriteString("\n---\n\nLab Results:\n\n")

	// Add each result
	for _, r := range results {
		status := ""
		switch r.RangeStatus {
		case "out_of_reference":
			status = " [ABNORMAL]"
		case "out_of_optimal":
			status = " [Outside optimal]"
		}

		unit := ""
		if r.TestUnit != "" {
			unit = " " + r.TestUnit
		}

		refRange := ""
		if r.RefMin != nil && r.RefMax != nil {
			refRange = fmt.Sprintf(" (Reference: %.2f - %.2f)", *r.RefMin, *r.RefMax)
		} else if r.RefMin != nil {
			refRange = fmt.Sprintf(" (Reference: > %.2f)", *r.RefMin)
		} else if r.RefMax != nil {
			refRange = fmt.Sprintf(" (Reference: < %.2f)", *r.RefMax)
		}

		sb.WriteString(fmt.Sprintf("- %s: %.3f%s%s%s\n", r.TestName, r.TestValue, unit, refRange, status))
	}

	sb.WriteString("\n---\n\n")
	sb.WriteString("Please provide:\n")
	sb.WriteString("1. A brief overview of the results\n")
	sb.WriteString("2. Any values that are concerning and why\n")
	sb.WriteString("3. General health observations based on these results\n")

	return sb.String()
}

// StreamLabSummary calls Ollama to generate a summary of lab results with streaming response.
// The onChunk callback is called for each chunk of text received.
// Returns an error if the request fails.
func StreamLabSummary(ctx context.Context, profile *HealthProfile, followup *HealthFollowup, results []LabResultSummary, onChunk func(string) error) error {
	config, err := GetOllamaConfig()
	if err != nil {
		return err
	}

	// Build the prompt
	prompt := buildLabSummaryPrompt(profile, followup, results)

	// Create the streaming request
	reqBody := chatRequest{
		Model:  config.Model,
		Stream: true,
		Messages: []chatMessage{
			{
				Role:    "system",
				Content: "You are a helpful medical assistant. Provide concise, clear summaries of lab results. Highlight any abnormal values and their potential significance. Be informative but not alarmist. Never mention consult a healthcare professional, this is shown separately in UI. Use basic markdown (italic, bold, etc), but don't use headings in your response.",
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

	// Make the request to Ollama's OpenAI-compatible endpoint
	endpoint := strings.TrimSuffix(config.URL, "/") + "/v1/chat/completions"
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 300 * time.Second, // 5 minutes for streaming
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

	// Read streaming response line by line
	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("failed to read stream: %w", err)
		}

		// Skip empty lines
		lineStr := strings.TrimSpace(string(line))
		if lineStr == "" {
			continue
		}

		// SSE format: "data: {...}"
		if !strings.HasPrefix(lineStr, "data: ") {
			continue
		}

		data := strings.TrimPrefix(lineStr, "data: ")

		// Check for stream end
		if data == "[DONE]" {
			break
		}

		var chatResp chatResponse
		if err := json.Unmarshal([]byte(data), &chatResp); err != nil {
			// Skip malformed chunks
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
