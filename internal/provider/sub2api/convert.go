package sub2api

import (
	"fmt"
	"math/rand"

	"github.com/sooneocean/uniapi/internal/provider"
)

// genID returns a short random hex string suitable for request/message IDs.
func genID() string {
	return fmt.Sprintf("%016x", rand.Uint64()) //nolint:gosec
}

// ---------------------------------------------------------------------------
// ChatGPT web API wire types
// ---------------------------------------------------------------------------

type chatgptContentPart struct {
	ContentType string `json:"content_type"`
	Parts       []string `json:"parts"`
}

type chatgptMessage struct {
	ID      string             `json:"id"`
	Author  chatgptAuthor      `json:"author"`
	Content chatgptContentPart `json:"content"`
}

type chatgptAuthor struct {
	Role string `json:"role"`
}

type chatgptRequest struct {
	Action          string           `json:"action"`
	Messages        []chatgptMessage `json:"messages"`
	Model           string           `json:"model"`
	ParentMessageID string           `json:"parent_message_id"`
	ConversationID  string           `json:"conversation_id,omitempty"`
}

// chatgptSSEData is the shape of the JSON payload inside each SSE "data:" line
// from the ChatGPT web backend.
type chatgptSSEData struct {
	Message struct {
		ID      string `json:"id"`
		Author  struct {
			Role string `json:"role"`
		} `json:"author"`
		Content struct {
			ContentType string   `json:"content_type"`
			Parts       []string `json:"parts"`
		} `json:"content"`
		Metadata struct {
			FinishDetails *struct {
				Type string `json:"type"`
			} `json:"finish_details"`
		} `json:"metadata"`
	} `json:"message"`
	Error *string `json:"error"`
}

// toChatGPTRequest converts a provider.ChatRequest to the ChatGPT web wire format.
func toChatGPTRequest(req *provider.ChatRequest) chatgptRequest {
	msgs := make([]chatgptMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		text := ""
		for _, block := range m.Content {
			text += block.Text
		}
		msgs = append(msgs, chatgptMessage{
			ID:     genID(),
			Author: chatgptAuthor{Role: m.Role},
			Content: chatgptContentPart{
				ContentType: "text",
				Parts:       []string{text},
			},
		})
	}
	return chatgptRequest{
		Action:          "next",
		Messages:        msgs,
		Model:           req.Model,
		ParentMessageID: genID(),
	}
}

// chatgptDataToResponse converts the last meaningful SSE payload into a
// provider.ChatResponse. It uses the accumulated text and stop reason.
func chatgptDataToResponse(data *chatgptSSEData, model string) *provider.ChatResponse {
	text := ""
	if len(data.Message.Content.Parts) > 0 {
		text = data.Message.Content.Parts[0]
	}
	stopReason := "stop"
	if data.Message.Metadata.FinishDetails != nil {
		stopReason = data.Message.Metadata.FinishDetails.Type
	}
	return &provider.ChatResponse{
		Content:    []provider.ContentBlock{{Type: "text", Text: text}},
		Model:      model,
		StopReason: stopReason,
	}
}

// chatgptDataToStreamEvent converts a chatgptSSEData frame into a StreamEvent.
func chatgptDataToStreamEvent(data *chatgptSSEData) *provider.StreamEvent {
	text := ""
	if len(data.Message.Content.Parts) > 0 {
		text = data.Message.Content.Parts[0]
	}
	return &provider.StreamEvent{
		Type:    "content_delta",
		Content: provider.ContentBlock{Type: "text", Text: text},
	}
}
