package livespeed

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/licoy/quosparr/internal/speed"
)

const tailBytes = 256 << 10

// Homes are the local log roots. Empty roots are skipped.
type Homes struct {
	Codex          string
	Grok           string
	ClaudeProjects []string
}

// Read collects responses and open requests whose logs changed recently.
func Read(now time.Time, homes Homes) ([]Response, []Flight) {
	if now.IsZero() {
		return nil, nil
	}
	since := now.Add(-fallbackWindow - time.Minute)
	var responses []Response
	var flights []Flight
	readGrok(now, homes.Grok, since, &responses, &flights)
	readCodex(now, homes.Codex, since, &responses, &flights)
	for _, root := range homes.ClaudeProjects {
		readClaude(now, root, since, &responses, &flights)
	}
	return responses, flights
}

func readGrok(now time.Time, home string, since time.Time, responses *[]Response, flights *[]Flight) {
	if home == "" {
		return
	}
	root := filepath.Join(home, "sessions")
	seen := make(map[string]bool)
	eachRecent(root, 4, since, func(path string, info fs.FileInfo) {
		if info.Name() != "updates.jsonl" && info.Name() != "events.jsonl" {
			return
		}
		dir := filepath.Dir(path)
		if seen[dir] {
			return
		}
		seen[dir] = true
		parseGrok(now, dir, tailOf(filepath.Join(dir, "updates.jsonl")), tailOf(filepath.Join(dir, "events.jsonl")), responses, flights)
	})
}

func readGrokHistory(now time.Time, home string) Response {
	if home == "" {
		return Response{}
	}
	type candidate struct {
		path string
		at   time.Time
	}
	var candidates []candidate
	eachRecent(filepath.Join(home, "sessions"), 4, now.Add(-grokHistoryWindow), func(path string, info fs.FileInfo) {
		if info.Name() == "updates.jsonl" {
			candidates = append(candidates, candidate{path: path, at: info.ModTime()})
		}
	})
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].at.After(candidates[j].at) })
	var responses []Response
	var flights []Flight
	for _, c := range candidates[:min(len(candidates), speed.SessionLimit)] {
		parseGrok(now, filepath.Dir(c.path), tailOf(c.path), nil, &responses, &flights)
	}
	return latestGrok(now, responses)
}

func readCodex(now time.Time, home string, since time.Time, responses *[]Response, flights *[]Flight) {
	if home == "" {
		return
	}
	eachRecent(filepath.Join(home, "sessions"), 6, since, func(path string, info fs.FileInfo) {
		if filepath.Ext(info.Name()) != ".jsonl" {
			return
		}
		parseCodex(now, path, tailOf(path), responses, flights)
	})
}

func readClaude(now time.Time, root string, since time.Time, responses *[]Response, flights *[]Flight) {
	eachRecent(root, 6, since, func(path string, info fs.FileInfo) {
		if filepath.Ext(info.Name()) != ".jsonl" {
			return
		}
		parseClaude(now, path, tailOf(path), responses, flights)
	})
}

func eachRecent(root string, maxDepth int, since time.Time, fn func(string, fs.FileInfo)) {
	if root == "" {
		return
	}
	seen := 0
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || seen > 20000 {
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		seen++
		if d.Type()&os.ModeSymlink != 0 {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		depth := 0
		if rel != "." {
			depth = len(splitPath(rel))
		}
		if d.IsDir() {
			if depth > maxDepth {
				return fs.SkipDir
			}
			return nil
		}
		if depth > maxDepth {
			return nil
		}
		info, err := d.Info()
		if err != nil || info.ModTime().Before(since) {
			return nil
		}
		fn(path, info)
		return nil
	})
}

func splitPath(path string) []string {
	var parts []string
	for {
		dir, base := filepath.Split(path)
		if base != "" {
			parts = append(parts, base)
		}
		if dir == "" || dir == path {
			break
		}
		path = filepath.Clean(dir)
	}
	return parts
}

func tailOf(path string) []byte {
	data, _, err := speed.ReadTail(path, tailBytes)
	if err != nil {
		return nil
	}
	return data
}

func parseGrok(now time.Time, dir string, updates, events []byte, responses *[]Response, flights *[]Flight) {
	model := ""
	var turnStart, waitAt, last time.Time
	open := false
	producing := false
	for _, line := range bytes.Split(events, []byte("\n")) {
		var ev struct {
			TS    string `json:"ts"`
			Type  string `json:"type"`
			Phase string `json:"phase"`
			Model string `json:"model_id"`
		}
		if json.Unmarshal(line, &ev) != nil || ev.TS == "" {
			continue
		}
		at, err := time.Parse(time.RFC3339Nano, ev.TS)
		if err != nil {
			continue
		}
		switch {
		case ev.Type == "turn_started":
			open = true
			producing = false
			turnStart, waitAt = at, time.Time{}
			if ev.Model != "" {
				model = ev.Model
			}
		case ev.Type == "first_token" || ev.Type == "phase_changed" &&
			(ev.Phase == "waiting_for_model" || ev.Phase == "streaming_reasoning" || ev.Phase == "streaming_text"):
			// A long turn's start can precede the bounded log tail.
			if !open {
				turnStart, open = at, true
			}
			producing, last = true, at
			if ev.Phase == "waiting_for_model" {
				waitAt = at
			}
		case ev.Type == "turn_ended":
			open, producing = false, false
			turnStart, waitAt = time.Time{}, time.Time{}
		case ev.Type == "phase_changed":
			producing = false
		}
	}
	for _, line := range bytes.Split(updates, []byte("\n")) {
		if !bytes.Contains(line, []byte(`"turn_completed"`)) {
			continue
		}
		var row grokUpdate
		if json.Unmarshal(line, &row) != nil || row.Params.Update.SessionUpdate != "turn_completed" || row.Params.Update.Usage == nil {
			continue
		}
		end := grokEnd(row)
		if end.IsZero() {
			continue
		}
		usage := row.Params.Update.Usage
		output := usage.OutputTokens
		if output == 0 {
			for _, bucket := range usage.ModelUsage {
				output += float64(clampTokens(bucket.OutputTokens))
			}
		}
		addGrokResponse(responses, dir, model, output, usage.APIDurationMs, end)
	}
	if !open || !producing || last.IsZero() || now.Sub(last) > flightFresh {
		return
	}
	start := turnStart
	if !waitAt.IsZero() {
		start = waitAt
	}
	*flights = append(*flights, Flight{Agent: dir, Model: model, Source: "grok", Start: start, Last: last})
}

type grokUpdate struct {
	Timestamp float64 `json:"timestamp"`
	Params    struct {
		Meta struct {
			AgentTimestampMs float64 `json:"agentTimestampMs"`
		} `json:"_meta"`
		Update struct {
			SessionUpdate string `json:"sessionUpdate"`
			Usage         *struct {
				OutputTokens  float64 `json:"outputTokens"`
				APIDurationMs float64 `json:"apiDurationMs"`
				ModelUsage    map[string]struct {
					OutputTokens float64 `json:"outputTokens"`
				} `json:"modelUsage"`
			} `json:"usage"`
		} `json:"update"`
	} `json:"params"`
}

func grokEnd(row grokUpdate) time.Time {
	if ms := row.Params.Meta.AgentTimestampMs; ms > 0 {
		return time.UnixMilli(int64(ms))
	}
	if row.Timestamp > 0 {
		return time.Unix(int64(row.Timestamp), 0)
	}
	return time.Time{}
}

func addGrokResponse(responses *[]Response, dir, model string, output, duration float64, end time.Time) {
	if duration <= 0 || duration > grokHistoryWindow.Seconds()*1000 {
		return
	}
	*responses = append(*responses, Response{
		Agent: dir, Model: model, Source: "grok", Output: clampTokens(output), Final: true,
		Start: end.Add(-time.Duration(duration) * time.Millisecond), End: end,
	})
}

func parseCodex(now time.Time, path string, data []byte, responses *[]Response, flights *[]Flight) {
	var request time.Time
	open := false
	streaming := false
	var model string
	var last time.Time
	for _, line := range bytes.Split(data, []byte("\n")) {
		if !bytes.Contains(line, []byte(`"type"`)) {
			continue
		}
		var row codexLine
		if json.Unmarshal(line, &row) != nil || row.Timestamp == "" {
			continue
		}
		at, err := time.Parse(time.RFC3339Nano, row.Timestamp)
		if err != nil {
			continue
		}
		last = at
		switch row.Type {
		case "turn_context":
			if row.Payload.Model != "" {
				model = row.Payload.Model
			}
		case "event_msg":
			switch row.Payload.Type {
			case "task_started":
				request, open, streaming = at, true, false
			case "item_completed":
				if open && !streaming && (row.Payload.Item.Type == "Reasoning" || row.Payload.Item.Type == "AgentMessage") {
					streaming = true
				}
			}
		case "response_item":
			if !streaming && (row.Payload.Type == "function_call_output" || row.Payload.Type == "custom_tool_call_output") {
				request, open = at, true
			}
		case "token_usage_record":
			if open && !request.IsZero() {
				*responses = append(*responses, Response{
					Agent: path, Model: model, Source: "codex", Output: clampTokens(float64(row.Payload.Usage.OutputTokens)),
					Final: true, Start: request, End: at,
				})
			}
			open, streaming = false, false
		}
	}
	if open && !request.IsZero() && !last.IsZero() && now.Sub(last) <= flightFresh {
		*flights = append(*flights, Flight{Agent: path, Model: model, Source: "codex", Start: request, Last: last})
	}
}

type codexLine struct {
	Timestamp string `json:"timestamp"`
	Type      string `json:"type"`
	Payload   struct {
		Type  string `json:"type"`
		Model string `json:"model"`
		Item  struct {
			Type string `json:"type"`
		} `json:"item"`
		Usage struct {
			OutputTokens int64 `json:"output_tokens"`
		} `json:"usage"`
	} `json:"payload"`
}

func parseClaude(now time.Time, path string, data []byte, responses *[]Response, flights *[]Flight) {
	var parent, last time.Time
	final := true
	var model string
	index := map[string]int{}
	for _, line := range bytes.Split(data, []byte("\n")) {
		if len(line) == 0 || line[0] != '{' {
			continue
		}
		var row claudeLine
		if json.Unmarshal(line, &row) != nil || row.Timestamp == "" {
			continue
		}
		at, err := time.Parse(time.RFC3339Nano, row.Timestamp)
		if err != nil {
			continue
		}
		last = at
		switch row.Type {
		case "user":
			parent = at
			final = true
		case "assistant":
			if row.Message.Usage.Output == 0 && row.Message.Model == "" {
				continue
			}
			done := stopSet(row.Message.Stop)
			start := parent
			if start.IsZero() || !at.After(start) {
				start = time.Time{}
			}
			resp := Response{
				Agent: path, Model: row.Message.Model, Source: "claude",
				Output: clampTokens(row.Message.Usage.Output), Final: done, Start: start, End: at,
			}
			if id := row.Message.ID; id != "" {
				if i, ok := index[id]; ok {
					(*responses)[i] = resp
				} else {
					index[id] = len(*responses)
					*responses = append(*responses, resp)
				}
			} else {
				*responses = append(*responses, resp)
			}
			if row.Message.Model != "" {
				model = row.Message.Model
			}
			final = done
		}
	}
	if final || parent.IsZero() || last.IsZero() || now.Sub(last) > flightFresh {
		return
	}
	*flights = append(*flights, Flight{Agent: path, Model: model, Source: "claude", Start: parent, Last: last})
}

type claudeLine struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	Message   struct {
		ID    string          `json:"id"`
		Model string          `json:"model"`
		Stop  json.RawMessage `json:"stop_reason"`
		Usage struct {
			Output float64 `json:"output_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

func stopSet(raw json.RawMessage) bool {
	raw = bytes.TrimSpace(raw)
	return len(raw) > 0 && !bytes.Equal(raw, []byte("null"))
}

func clampTokens(v float64) int64 {
	if v <= 0 {
		return 0
	}
	if v > 1e12 {
		return 1e12
	}
	return int64(v)
}
