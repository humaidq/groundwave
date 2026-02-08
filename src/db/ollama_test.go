// SPDX-FileCopyrightText: 2025 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestOllamaStreaming(t *testing.T) {
	resetDatabase(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"Hello \"}}]}\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"world\"}}]}\n"))
		_, _ = w.Write([]byte("data: [DONE]\n"))
	}))
	defer server.Close()

	t.Setenv("OLLAMA_URL", server.URL)
	t.Setenv("OLLAMA_MODEL", "test-model")

	profile := &HealthProfile{Name: "Test"}
	followup := &HealthFollowup{FollowupDate: time.Now(), HospitalName: "Clinic"}
	results := []LabResultSummary{{TestName: "Glucose", TestValue: 5.5, TestUnit: "mmol/L"}}

	prompt := buildLabSummaryPrompt(profile, followup, results)
	if prompt == "" {
		t.Fatalf("expected prompt")
	}

	var streamed string
	if err := streamChatCompletion(context.Background(), "system", "user", func(chunk string) error {
		streamed += chunk
		return nil
	}); err != nil {
		t.Fatalf("streamChatCompletion failed: %v", err)
	}
	if streamed != "Hello world" {
		t.Fatalf("expected streamed output, got %q", streamed)
	}

	streamed = ""
	if err := StreamLabSummary(context.Background(), profile, followup, results, func(chunk string) error {
		streamed += chunk
		return nil
	}); err != nil {
		t.Fatalf("StreamLabSummary failed: %v", err)
	}
	if streamed != "Hello world" {
		t.Fatalf("expected streamed output, got %q", streamed)
	}

	contact := &Contact{NameDisplay: "Chat Person"}
	chats := []ContactChat{{Sender: ChatSenderThem, Message: "Hi", SentAt: time.Now()}}
	streamed = ""
	if err := StreamContactChatSummary(context.Background(), contact, chats, func(chunk string) error {
		streamed += chunk
		return nil
	}); err != nil {
		t.Fatalf("StreamContactChatSummary failed: %v", err)
	}
	if streamed != "Hello world" {
		t.Fatalf("expected streamed output, got %q", streamed)
	}

	zkNotes := []ZKChatNote{{ID: "note-1", Title: "Note", Content: "Content"}}
	streamed = ""
	if err := StreamZKChat(context.Background(), zkNotes, "Question", func(chunk string) error {
		streamed += chunk
		return nil
	}); err != nil {
		t.Fatalf("StreamZKChat failed: %v", err)
	}
	if streamed != "Hello world" {
		t.Fatalf("expected streamed output, got %q", streamed)
	}
}
