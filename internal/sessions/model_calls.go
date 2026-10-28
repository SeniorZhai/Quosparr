package sessions

import (
	"encoding/json"
	"strconv"
)

// modelCallCounter counts usage records, not assistant text or tool events.
// Modern Codex response records take precedence over mirrored token events.
type modelCallCounter struct {
	seen                      map[string]bool
	modern, legacy            int
	hasModern, known, partial bool
}

func (c *modelCallCounter) add(provider string, obj map[string]any) {
	key, count, modern := "", 1, false
	payload := managedObject(obj["payload"])
	switch provider {
	case ProviderCodex:
		switch {
		case managedString(obj["type"]) == "token_usage_record":
			c.hasModern = true
			if managedObject(payload["usage"]) == nil {
				c.partial = true
				return
			}
			modern = true
			key = managedString(payload["response_id"])
			if key == "" {
				raw, _ := json.Marshal(obj)
				key = string(raw)
			}
		case managedString(obj["type"]) == "event_msg" && managedString(payload["type"]) == "token_count":
			info := managedObject(payload["info"])
			usage := managedObject(info["total_token_usage"])
			if usage == nil {
				return
			}
			total, err := tokenInteger(usage, "total_tokens")
			if err != nil || total < 0 {
				c.partial = true
				return
			}
			if total == 0 {
				return
			}
			raw, _ := json.Marshal(usage)
			key = string(raw)
		default:
			return
		}
	case ProviderClaude:
		if managedString(obj["type"]) != "assistant" {
			return
		}
		message := managedObject(obj["message"])
		if managedObject(message["usage"]) == nil {
			return
		}
		key = managedString(message["id"])
		if key == "" {
			c.partial = true
			return
		}
	case ProviderGrok:
		update := managedObject(managedObject(obj["params"])["update"])
		if managedString(update["sessionUpdate"]) != "turn_completed" {
			return
		}
		usage := managedObject(update["usage"])
		if _, ok := usage["modelCalls"]; !ok {
			c.partial = true
			return
		}
		var err error
		count, err = tokenInteger(usage, "modelCalls")
		if err != nil || count < 0 {
			c.partial = true
			return
		}
		key = managedString(update["turnId"])
		if key == "" {
			raw, _ := json.Marshal(obj)
			key = string(raw)
		}
	default:
		return
	}
	key = strconv.FormatBool(modern) + ":" + key
	if c.seen == nil {
		c.seen = make(map[string]bool)
	}
	if c.seen[key] {
		return
	}
	c.seen[key] = true
	c.known = true
	target := &c.legacy
	if modern {
		target = &c.modern
	}
	sum, err := checkedAdd(*target, count, "model calls")
	if err != nil {
		c.partial = true
		return
	}
	*target = sum
}

func (c *modelCallCounter) merge(detail *ManagedDetail) {
	detail.ModelCallsPartial = detail.ModelCallsPartial || c.partial
	if !c.known {
		return
	}
	count := c.legacy
	if c.hasModern {
		count = c.modern
	}
	if detail.ModelCalls == nil {
		detail.ModelCalls = new(int)
	}
	total, err := checkedAdd(*detail.ModelCalls, count, "model calls")
	if err != nil {
		detail.ModelCallsPartial = true
		return
	}
	*detail.ModelCalls = total
}
