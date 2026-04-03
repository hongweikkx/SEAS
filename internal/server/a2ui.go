package server

import (
	"encoding/json"
	"fmt"
	"net/http"
)

const a2uiEventContentType = "text/event-stream"

type A2UIMessage struct {
	BeginRendering   *BeginRenderingMsg   `json:"beginRendering,omitempty"`
	SurfaceUpdate    *SurfaceUpdateMsg    `json:"surfaceUpdate,omitempty"`
	DataModelUpdate  *DataModelUpdateMsg  `json:"dataModelUpdate,omitempty"`
	InterruptRequest *InterruptRequestMsg `json:"interruptRequest,omitempty"`
	Error            *A2UIErrorMsg        `json:"error,omitempty"`
}

type BeginRenderingMsg struct {
	SurfaceID       string `json:"surfaceId"`
	RootComponentID string `json:"rootComponentId"`
	Title           string `json:"title,omitempty"`
}

type SurfaceUpdateMsg struct {
	SurfaceID  string          `json:"surfaceId"`
	Components []A2UIComponent `json:"components"`
}

type DataModelUpdateMsg struct {
	SurfaceID string         `json:"surfaceId"`
	Data      map[string]any `json:"data"`
}

type InterruptRequestMsg struct {
	SurfaceID   string                 `json:"surfaceId"`
	InterruptID string                 `json:"interruptId,omitempty"`
	Prompt      string                 `json:"prompt,omitempty"`
	Contexts    []A2UIInterruptContext `json:"contexts,omitempty"`
}

type A2UIInterruptContext struct {
	ID      string `json:"id,omitempty"`
	Name    string `json:"name,omitempty"`
	Address string `json:"address,omitempty"`
	Type    string `json:"type,omitempty"`
}

type A2UIErrorMsg struct {
	Message string `json:"message"`
}

type A2UIComponent struct {
	ID       string         `json:"id"`
	Type     string         `json:"type"`
	Props    map[string]any `json:"props,omitempty"`
	Children []string       `json:"children,omitempty"`
}

func writeSSEHeaders(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Content-Type", a2uiEventContentType)
	h.Set("Cache-Control", "no-cache, no-transform")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no")
}

func writeSSEMessage(w http.ResponseWriter, msg A2UIMessage) error {
	payload, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	if _, err := fmt.Fprintf(w, "data: %s\n\n", payload); err != nil {
		return err
	}
	return nil
}

func surfaceComponentText(id string, props map[string]any, children ...string) A2UIComponent {
	return A2UIComponent{
		ID:       id,
		Type:     "text",
		Props:    props,
		Children: children,
	}
}

func surfaceComponentLayout(typ, id string, props map[string]any, children ...string) A2UIComponent {
	return A2UIComponent{
		ID:       id,
		Type:     typ,
		Props:    props,
		Children: children,
	}
}

func blocksToA2UIComponents(blocks []map[string]any, prefix string) []A2UIComponent {
	components := make([]A2UIComponent, 0, len(blocks))
	for index, block := range blocks {
		componentID := fmt.Sprintf("%s-%d", prefix, index)
		componentType, _ := block["type"].(string)
		switch componentType {
		case "table":
			components = append(components, A2UIComponent{
				ID:    componentID,
				Type:  "table",
				Props: cloneMap(block),
			})
		case "chart":
			components = append(components, A2UIComponent{
				ID:    componentID,
				Type:  "chart",
				Props: cloneMap(block),
			})
		case "text":
			components = append(components, A2UIComponent{
				ID:    componentID,
				Type:  "text",
				Props: cloneMap(block),
			})
		default:
			components = append(components, A2UIComponent{
				ID:    componentID,
				Type:  "text",
				Props: map[string]any{"content": fmt.Sprintf("%v", block)},
			})
		}
	}
	return components
}

func cloneMap(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}

	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
