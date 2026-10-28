package sessions

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestManagedDetailModelCalls(t *testing.T) {
	cases := []struct {
		name, provider, body string
		count                int
		known, partial       bool
	}{
		{"codex mirrors", ProviderCodex,
			`{"type":"token_usage_record","payload":{"response_id":"one","usage":{"output_tokens":3}}}
{"type":"token_usage_record","payload":{"response_id":"one","usage":{"output_tokens":4}}}
{"type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"total_tokens":7}}}}
{"type":"token_usage_record","payload":{"response_id":"two","usage":{"output_tokens":5}}}
{"type":"response_item","payload":{"type":"function_call","name":"exec"}}
`, 2, true, false},
		{"codex legacy", ProviderCodex,
			`{"type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"total_tokens":7}}}}
{"type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"total_tokens":7}}}}
{"type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"total_tokens":10}}}}
`, 2, true, false},
		{"claude fragments", ProviderClaude,
			`{"type":"assistant","uuid":"a","message":{"id":"msg-1","usage":{"output_tokens":1}}}
{"type":"assistant","uuid":"b","message":{"id":"msg-1","usage":{"output_tokens":9}}}
{"type":"assistant","message":{"id":"msg-2","usage":{"output_tokens":5}}}
{"type":"user","message":{"content":"tool result"}}
`, 2, true, false},
		{"grok calls not turns", ProviderGrok,
			`{"timestamp":1,"params":{"update":{"sessionUpdate":"turn_completed","usage":{"modelCalls":4}}}}
{"timestamp":1,"params":{"update":{"sessionUpdate":"turn_completed","usage":{"modelCalls":4}}}}
{"timestamp":2,"params":{"update":{"sessionUpdate":"turn_completed","usage":{"modelCalls":2}}}}
`, 6, true, false},
		{"grok invalid", ProviderGrok, `{"params":{"update":{"sessionUpdate":"turn_completed","usage":{"modelCalls":-1}}}}`, 0, false, true},
		{"text is not a call", ProviderClaude, `{"type":"assistant","message":{"content":"answer"}}`, 0, false, false},
		{"unknown claude id", ProviderClaude, `{"type":"assistant","message":{"usage":{"output_tokens":1}}}`, 0, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "log.jsonl")
			writeManagedFixture(t, path, tc.body)
			file, err := SnapshotManagedFile(root, path)
			if err != nil {
				t.Fatal(err)
			}
			detail, err := ReadManagedDetail(context.Background(), ManagedSession{Provider: tc.provider, Root: root, Files: []ManagedFile{file}})
			if err != nil {
				t.Fatal(err)
			}
			if (detail.ModelCalls != nil) != tc.known || detail.ModelCallsPartial != tc.partial {
				t.Fatalf("unexpected count: %+v", detail)
			}
			if tc.known && *detail.ModelCalls != tc.count {
				t.Fatalf("calls=%d want %d", *detail.ModelCalls, tc.count)
			}
		})
	}
}

func TestManagedModelCallsMarksSkippedRecords(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "log.jsonl")
	writeManagedFixture(t, path, `{"type":"token_usage_record","payload":{"response_id":"one","usage":{}}}`+"\n"+strings.Repeat("x", managedLineBytes+1)+"\n")
	file, _ := SnapshotManagedFile(root, path)
	detail, err := ReadManagedDetail(context.Background(), ManagedSession{Provider: ProviderCodex, Root: root, Files: []ManagedFile{file}})
	if err != nil || detail.ModelCalls == nil || *detail.ModelCalls != 1 || !detail.ModelCallsPartial {
		t.Fatalf("detail=%+v err=%v", detail, err)
	}
}

func TestManagedModelCallsDeduplicatesAcrossFiles(t *testing.T) {
	root := t.TempDir()
	session := ManagedSession{Provider: ProviderClaude, Root: root}
	for _, name := range []string{"main.jsonl", "companion.jsonl"} {
		path := filepath.Join(root, name)
		writeManagedFixture(t, path, `{"type":"assistant","message":{"id":"same-response","usage":{"output_tokens":3}}}`+"\n")
		file, err := SnapshotManagedFile(root, path)
		if err != nil {
			t.Fatal(err)
		}
		session.Files = append(session.Files, file)
	}
	detail, err := ReadManagedDetail(context.Background(), session)
	if err != nil || detail.ModelCalls == nil || *detail.ModelCalls != 1 {
		t.Fatalf("detail=%+v err=%v", detail, err)
	}
}
