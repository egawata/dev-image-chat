package main

import (
	"encoding/json"
	"strings"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// rawEntry represents a single line in the JSONL log.
type rawEntry struct {
	Type    string          `json:"type"`
	Message json.RawMessage `json:"message"`
}

// rawMessage is the message field inside a rawEntry.
type rawMessage struct {
	Content json.RawMessage `json:"content"`
}

// contentBlock represents one element of the assistant's content array.
type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// ParseJSONL parses JSONL bytes and extracts user/assistant conversation messages.
func ParseJSONL(data []byte) []Message {
	var messages []Message

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var entry rawEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}

		switch entry.Type {
		case "user":
			msg := parseUserEntry(entry.Message)
			if msg != nil {
				messages = append(messages, *msg)
			}
		case "assistant":
			msg := parseAssistantEntry(entry.Message)
			if msg != nil {
				messages = append(messages, *msg)
			}
		}
	}

	return messages
}

func parseUserEntry(raw json.RawMessage) *Message {
	if raw == nil {
		return nil
	}

	var msg rawMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		return nil
	}

	// content is a plain string → user's direct input
	var strContent string
	if err := json.Unmarshal(msg.Content, &strContent); err == nil {
		strContent = strings.TrimSpace(strContent)
		if strContent != "" {
			return &Message{Role: "user", Content: strContent}
		}
		return nil
	}

	// content is an array → likely tool_result, skip
	return nil
}

func parseAssistantEntry(raw json.RawMessage) *Message {
	if raw == nil {
		return nil
	}

	var msg rawMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		return nil
	}

	// content should be an array of blocks
	var blocks []contentBlock
	if err := json.Unmarshal(msg.Content, &blocks); err != nil {
		return nil
	}

	var textParts []string
	for _, b := range blocks {
		if b.Type == "text" && strings.TrimSpace(b.Text) != "" {
			textParts = append(textParts, strings.TrimSpace(b.Text))
		}
	}

	if len(textParts) == 0 {
		return nil
	}

	return &Message{
		Role:    "assistant",
		Content: strings.Join(textParts, "\n"),
	}
}

// TailMessages returns the last n messages from the slice.
func TailMessages(msgs []Message, n int) []Message {
	if len(msgs) <= n {
		return msgs
	}
	return msgs[len(msgs)-n:]
}
