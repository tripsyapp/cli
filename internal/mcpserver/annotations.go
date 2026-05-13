package mcpserver

import (
	"bytes"
	"encoding/json"
	"strings"
)

func NormalizeToolAnnotationsPayload(payload []byte) []byte {
	if len(payload) == 0 || !bytes.Contains(payload, []byte(`"tools"`)) {
		return payload
	}
	if json.Valid(payload) {
		if normalized, ok := normalizeToolAnnotationsJSON(payload); ok {
			return normalized
		}
		return payload
	}

	lines := bytes.SplitAfter(payload, []byte("\n"))
	changed := false
	for i, line := range lines {
		trimmed := bytes.TrimRight(line, "\r\n")
		data, ok := bytes.CutPrefix(trimmed, []byte("data: "))
		if !ok || !json.Valid(data) || !bytes.Contains(data, []byte(`"tools"`)) {
			continue
		}
		normalized, ok := normalizeToolAnnotationsJSON(data)
		if !ok {
			continue
		}
		suffix := line[len(trimmed):]
		lines[i] = append(append([]byte("data: "), normalized...), suffix...)
		changed = true
	}
	if !changed {
		return payload
	}
	return bytes.Join(lines, nil)
}

func normalizeToolAnnotationsJSON(payload []byte) ([]byte, bool) {
	var root any
	if err := json.Unmarshal(payload, &root); err != nil {
		return nil, false
	}
	if !normalizeToolAnnotationsValue(root) {
		return nil, false
	}
	normalized, err := json.Marshal(root)
	if err != nil {
		return nil, false
	}
	return normalized, true
}

func normalizeToolAnnotationsValue(value any) bool {
	object, ok := value.(map[string]any)
	if !ok {
		return false
	}
	changed := false
	if result, ok := object["result"]; ok {
		changed = normalizeToolAnnotationsValue(result) || changed
	}
	tools, ok := object["tools"].([]any)
	if !ok {
		return changed
	}
	for _, item := range tools {
		tool, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name, _ := tool["name"].(string)
		annotations, _ := tool["annotations"].(map[string]any)
		if annotations == nil {
			annotations = map[string]any{}
			tool["annotations"] = annotations
			changed = true
		}
		if _, ok := annotations["readOnlyHint"]; !ok {
			annotations["readOnlyHint"] = readOnlyToolName(name)
			changed = true
		}
		if _, ok := annotations["openWorldHint"]; !ok {
			annotations["openWorldHint"] = false
			changed = true
		}
		if _, ok := annotations["destructiveHint"]; !ok {
			annotations["destructiveHint"] = destructiveToolName(name)
			changed = true
		}
	}
	return changed
}

func readOnlyToolName(name string) bool {
	return strings.HasSuffix(name, "_list") ||
		strings.HasSuffix(name, "_show") ||
		name == toolName("tripsy", "status") ||
		name == toolName("tripsy", "itinerary", "guidance") ||
		name == toolName("tripsy", "collaborators", "list")
}

func destructiveToolName(name string) bool {
	return strings.HasSuffix(name, "_delete") || name == toolName("tripsy", "raw_request")
}
