package agents

import (
	"encoding/json"
	"fmt"
)

// responseTypeField is used for initial unmarshalling to read the type discriminator.
type responseTypeField struct {
	Type string `json:"type"`
}

// ParseResponse attempts to unmarshal the JSON string into the appropriate
// response type based on the "type" field.
// Returns the parsed value and nil error, or nil and an error.
func ParseResponse(jsonStr string) (any, error) {
	raw := []byte(jsonStr)

	var probe responseTypeField
	if err := json.Unmarshal(raw, &probe); err != nil {
		return nil, fmt.Errorf("agents: failed to parse response JSON: %w", err)
	}

	if probe.Type == "" {
		return nil, fmt.Errorf("agents: response JSON missing \"type\" field")
	}

	switch probe.Type {
	case "clarifications":
		var v ClarificationsResponse
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, fmt.Errorf("agents: failed to parse clarifications response: %w", err)
		}
		return v, nil

	case "options":
		var v OptionsResponse
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, fmt.Errorf("agents: failed to parse options response: %w", err)
		}
		return v, nil

	case "execution_brief":
		var v ExecutionBrief
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, fmt.Errorf("agents: failed to parse execution_brief response: %w", err)
		}
		return v, nil

	case "boss_plan":
		var v BossPlan
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, fmt.Errorf("agents: failed to parse boss_plan response: %w", err)
		}
		return v, nil

	case "spawn_workers":
		var v SpawnWorkersRequest
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, fmt.Errorf("agents: failed to parse spawn_workers response: %w", err)
		}
		return v, nil

	case "boss_summary":
		var v BossSummary
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, fmt.Errorf("agents: failed to parse boss_summary response: %w", err)
		}
		return v, nil

	case "worker_result":
		var v WorkerResult
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, fmt.Errorf("agents: failed to parse worker_result response: %w", err)
		}
		return v, nil

	default:
		return nil, fmt.Errorf("agents: unknown response type %q", probe.Type)
	}
}
