/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package db

import (
	"context"
	"fmt"
	"strings"
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
	prompt := buildZKChatPrompt(notes, message)

	systemPrompt := "You are a research assistant for a personal zettelkasten. Use the provided notes as primary sources. If details are missing, say so clearly. Cite note titles with inline links to /zk/<id>. Prefer org-roam format [[id:UUID][Title]], but [Title](/zk/UUID) is also acceptable. Provide structured, concise responses. Use markdown for emphasis and lists, but avoid headings."

	return streamChatCompletion(ctx, systemPrompt, prompt, onChunk)
}
