package provider

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

	"brutus/tools"
)

// Saturn implements Provider using Saturn-discovered services.
// Saturn provides zero-config AI service discovery on local networks.
// Any beacon on the network can provide credentials automatically.
type Saturn struct {
	service    *SaturnService
	httpClient *http.Client
	model      string
	maxTokens  int
}

// SaturnConfig holds configuration for Saturn discovery.
type SaturnConfig struct {
	DiscoveryTimeout time.Duration // How long to search for services
	Model            string        // Model to request (if supported)
	MaxTokens        int
}

// NewSaturn discovers Saturn services and creates a provider.
// Returns error if no services are found.
func NewSaturn(ctx context.Context, cfg SaturnConfig) (*Saturn, error) {
	if cfg.DiscoveryTimeout == 0 {
		cfg.DiscoveryTimeout = 3 * time.Second
	}

	services, err := DiscoverSaturn(ctx, cfg.DiscoveryTimeout)
	if err != nil {
		return nil, fmt.Errorf("saturn discovery failed: %w", err)
	}

	if len(services) == 0 {
		return nil, fmt.Errorf("no saturn services found on network")
	}

	// Use highest priority (lowest number) service
	svc := services[0]

	// Verify service is healthy
	if err := healthCheck(svc); err != nil {
		// Try next service
		for _, s := range services[1:] {
			if healthCheck(s) == nil {
				svc = s
				break
			}
		}
	}

	return &Saturn{
		service:    &svc,
		httpClient: &http.Client{Timeout: 120 * time.Second},
		model:      cfg.Model,
		maxTokens:  cfg.MaxTokens,
	}, nil
}

func (s *Saturn) Name() string {
	return fmt.Sprintf("saturn(%s)", s.service.Name)
}

func (s *Saturn) GetModel() string {
	return s.model
}

func (s *Saturn) SetModel(model string) {
	s.model = model
}

func (s *Saturn) GetService() *SaturnService {
	return s.service
}

func (s *Saturn) ListModels(ctx context.Context) ([]ModelInfo, error) {
	httpReq, err := http.NewRequestWithContext(ctx, "GET", s.service.URL()+"/v1/models", nil)
	if err != nil {
		return nil, err
	}

	if s.service.EphemeralKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+s.service.EphemeralKey)
	}

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var modelsResp struct {
		Data []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, err
	}

	var models []ModelInfo
	for _, m := range modelsResp.Data {
		name := m.Name
		if name == "" {
			name = m.ID
		}
		models = append(models, ModelInfo{ID: m.ID, Name: name})
	}

	return models, nil
}

// Chat implements the Provider interface using OpenAI-compatible API.
func (s *Saturn) Chat(ctx context.Context, systemPrompt string, messages []Message, toolDefs []tools.Tool) (Message, error) {
	// Build OpenAI-format request
	req := openAIRequest{
		Model:     s.model,
		MaxTokens: s.maxTokens,
		Messages:  convertToOpenAIMessages(systemPrompt, messages),
		Tools:     convertToOpenAITools(toolDefs),
	}

	// Make the API call
	body, err := json.Marshal(req)
	if err != nil {
		return Message{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		s.service.URL()+"/v1/chat/completions",
		bytes.NewReader(body))
	if err != nil {
		return Message{}, err
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// Use ephemeral key from beacon if available
	if s.service.EphemeralKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+s.service.EphemeralKey)
	}

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return Message{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return Message{}, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var openAIResp openAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&openAIResp); err != nil {
		return Message{}, err
	}

	return convertFromOpenAIResponse(openAIResp), nil
}

func (s *Saturn) ChatStream(ctx context.Context, systemPrompt string, messages []Message, toolDefs []tools.Tool) (<-chan StreamDelta, error) {
	req := openAIRequest{
		Model:     s.model,
		MaxTokens: s.maxTokens,
		Messages:  convertToOpenAIMessages(systemPrompt, messages),
		Tools:     convertToOpenAITools(toolDefs),
		Stream:    true,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		s.service.URL()+"/v1/chat/completions",
		bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	if s.service.EphemeralKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+s.service.EphemeralKey)
	}

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	ch := make(chan StreamDelta, 10)
	go s.processStream(ctx, resp, ch)
	return ch, nil
}

func (s *Saturn) processStream(ctx context.Context, resp *http.Response, ch chan<- StreamDelta) {
	defer resp.Body.Close()
	defer close(ch)

	reader := bufio.NewReader(resp.Body)
	var accumulatedToolCalls []ToolCall

	for {
		select {
		case <-ctx.Done():
			ch <- StreamDelta{Error: ctx.Err(), Done: true}
			return
		default:
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				ch <- StreamDelta{Error: err, Done: true}
			} else {
				ch <- StreamDelta{Done: true}
			}
			return
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			ch <- StreamDelta{Done: true}
			return
		}

		var chunk openAIStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if len(chunk.Choices) == 0 {
			continue
		}

		delta := chunk.Choices[0].Delta

		if delta.Content != "" {
			ch <- StreamDelta{Content: delta.Content}
		}

		for _, tc := range delta.ToolCalls {
			for len(accumulatedToolCalls) <= tc.Index {
				accumulatedToolCalls = append(accumulatedToolCalls, ToolCall{})
			}
			if tc.ID != "" {
				accumulatedToolCalls[tc.Index].ID = tc.ID
			}
			if tc.Function.Name != "" {
				accumulatedToolCalls[tc.Index].Name = tc.Function.Name
			}
			if tc.Function.Arguments != "" {
				current := string(accumulatedToolCalls[tc.Index].Input)
				accumulatedToolCalls[tc.Index].Input = json.RawMessage(current + tc.Function.Arguments)
			}
			ch <- StreamDelta{ToolCall: &accumulatedToolCalls[tc.Index]}
		}

		if chunk.Choices[0].FinishReason != "" {
			ch <- StreamDelta{Done: true}
			return
		}
	}
}

func healthCheck(svc SaturnService) error {
	if svc.APIBase != "" {
		return nil // Remote APIs don't have health endpoints
	}

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(svc.URL() + "/v1/health")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed: %d", resp.StatusCode)
	}
	return nil
}

// OpenAI-compatible types

type openAIRequest struct {
	Model     string          `json:"model,omitempty"`
	MaxTokens int             `json:"max_tokens,omitempty"`
	Messages  []openAIMessage `json:"messages"`
	Tools     []openAITool    `json:"tools,omitempty"`
	Stream    bool            `json:"stream,omitempty"`
}

type openAIMessage struct {
	Role       string           `json:"role"`
	Content    any              `json:"content,omitempty"` // string or []contentPart
	ToolCalls  []openAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
}

type openAIToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type openAITool struct {
	Type     string `json:"type"`
	Function struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		Parameters  json.RawMessage `json:"parameters"`
	} `json:"function"`
}

type openAIResponse struct {
	Choices []struct {
		Message openAIMessage `json:"message"`
	} `json:"choices"`
}

type openAIStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content   string `json:"content"`
			ToolCalls []struct {
				Index    int    `json:"index"`
				ID       string `json:"id"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

func convertToOpenAIMessages(systemPrompt string, messages []Message) []openAIMessage {
	result := []openAIMessage{{
		Role:    "system",
		Content: systemPrompt,
	}}

	for _, msg := range messages {
		if len(msg.ToolResults) > 0 {
			// Tool results
			for _, tr := range msg.ToolResults {
				result = append(result, openAIMessage{
					Role:       "tool",
					Content:    tr.Content,
					ToolCallID: tr.ID,
				})
			}
		} else if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			// Assistant with tool calls
			var toolCalls []openAIToolCall
			for _, tc := range msg.ToolCalls {
				toolCalls = append(toolCalls, openAIToolCall{
					ID:   tc.ID,
					Type: "function",
					Function: struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					}{
						Name:      tc.Name,
						Arguments: string(tc.Input),
					},
				})
			}
			result = append(result, openAIMessage{
				Role:      "assistant",
				Content:   msg.Content,
				ToolCalls: toolCalls,
			})
		} else {
			// Regular message
			result = append(result, openAIMessage{
				Role:    msg.Role,
				Content: msg.Content,
			})
		}
	}

	return result
}

func convertToOpenAITools(toolDefs []tools.Tool) []openAITool {
	result := make([]openAITool, 0, len(toolDefs))
	for _, t := range toolDefs {
		// Convert Anthropic schema to OpenAI format
		params, _ := json.Marshal(map[string]any{
			"type":       "object",
			"properties": t.InputSchema.Properties,
		})

		tool := openAITool{Type: "function"}
		tool.Function.Name = t.Name
		tool.Function.Description = t.Description
		tool.Function.Parameters = params
		result = append(result, tool)
	}
	return result
}

func convertFromOpenAIResponse(resp openAIResponse) Message {
	if len(resp.Choices) == 0 {
		return Message{Role: "assistant"}
	}

	choice := resp.Choices[0].Message
	msg := Message{
		Role: "assistant",
	}

	// Handle content (might be string or structured)
	if content, ok := choice.Content.(string); ok {
		msg.Content = content
	}

	// Handle tool calls
	for _, tc := range choice.ToolCalls {
		msg.ToolCalls = append(msg.ToolCalls, ToolCall{
			ID:    tc.ID,
			Name:  tc.Function.Name,
			Input: json.RawMessage(tc.Function.Arguments),
		})
	}

	return msg
}
