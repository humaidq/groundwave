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

func buildContactChatSummaryPrompt(contact *Contact, chats []ContactChat) string {
	var sb strings.Builder

	sb.WriteString("Summarize the following conversation history from the last 48 hours. ")
	sb.WriteString("Write a concise single-paragraph summary suitable for a CRM contact log entry.\n\n")
	sb.WriteString(fmt.Sprintf("Contact: %s\n\n", contact.NameDisplay))
	sb.WriteString("Conversation:\n")

	for _, chat := range chats {
		sender := "Them"
		switch chat.Sender {
		case ChatSenderMe:
			sender = "Me"
		case ChatSenderMix:
			sender = "Mixed"
		}

		sb.WriteString(fmt.Sprintf("[%s] %s: %s\n", chat.SentAt.Format("Jan 2, 2006 3:04 PM"), sender, chat.Message))
	}

	return sb.String()
}

// StreamContactChatSummary summarizes recent chat history for a contact using Ollama.
func StreamContactChatSummary(ctx context.Context, contact *Contact, chats []ContactChat, onChunk func(string) error) error {
	prompt := buildContactChatSummaryPrompt(contact, chats)
	systemPrompt := "You summarize conversation histories for a personal CRM. Provide a concise single-paragraph summary for a contact log entry. Do not use headings or bullet points."

	return streamChatCompletion(ctx, systemPrompt, prompt, onChunk)
}
