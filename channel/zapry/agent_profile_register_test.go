package zapry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type capturedProfileRequest struct {
	Path string
	Body map[string]interface{}
}

func buildTestProfileSource() *ProfileSource {
	skillContent := `---
skillKey: tarot-reading
skillVersion: 1.0.0
---
# Tarot Reading

Body`
	skill := ProfileSourceSkill{
		SkillKey:     "tarot-reading",
		SkillVersion: "1.0.0",
		Path:         "skills/tarot-reading/SKILL.md",
		Content:      skillContent,
		SHA256:       sha256Hex([]byte(skillContent)),
		Bytes:        len(skillContent),
	}
	snapshotID, _ := ComputeSnapshotID("# 林晚晴\n\n温柔理性。", []ProfileSourceSkill{skill})
	return &ProfileSource{
		Version:    "v1",
		Source:     "code",
		AgentKey:   "agents-demo-wanqing",
		SnapshotID: snapshotID,
		SoulMD:     "# 林晚晴\n\n温柔理性。",
		Skills:     []ProfileSourceSkill{skill},
	}
}

func newTestZapryAgentForProfile(serverURL string, source *ProfileSource) *ZapryAgent {
	return &ZapryAgent{
		Config: &AgentConfig{
			Platform:      "zapry",
			APIBaseURL:    serverURL + "/bot",
			BotToken:      "test-token",
			ProfileSource: source,
		},
		Bot: &AgentAPI{
			Client: http.DefaultClient,
		},
	}
}

func TestRegisterProfileUnsupportedDoesNotFallbackToLegacy(t *testing.T) {
	requests := make([]capturedProfileRequest, 0, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body failed: %v", err)
		}
		requests = append(requests, capturedProfileRequest{
			Path: r.URL.Path,
			Body: body,
		})

		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"ok":false,"unsupported_profile_source":true}`))
	}))
	defer server.Close()

	agent := newTestZapryAgentForProfile(server.URL, buildTestProfileSource())
	agent.Bot.Client = server.Client()
	agent.registerProfile()

	if len(requests) != 1 {
		t.Fatalf("expected exactly one profileSource request, got %d", len(requests))
	}
	if _, ok := requests[0].Body["profileSource"]; !ok {
		t.Fatalf("request should carry profileSource")
	}
	if _, ok := requests[0].Body["skills"]; ok {
		t.Fatalf("request should not carry legacy skills")
	}
	if _, ok := requests[0].Body["persona"]; ok {
		t.Fatalf("request should not carry legacy persona")
	}
}

func TestRegisterProfileExtendedSuccessSingleRequest(t *testing.T) {
	requests := make([]capturedProfileRequest, 0, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body failed: %v", err)
		}
		requests = append(requests, capturedProfileRequest{Path: r.URL.Path, Body: body})

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true,"derived":{"snapshotId":"abc","profile":{"skills":["tarot-reading"]}}}`))
	}))
	defer server.Close()

	agent := newTestZapryAgentForProfile(server.URL, buildTestProfileSource())
	agent.Bot.Client = server.Client()
	agent.registerProfile()

	if len(requests) != 1 {
		t.Fatalf("expected only one request on extended success, got %d", len(requests))
	}
	if _, ok := requests[0].Body["skills"]; ok {
		t.Fatalf("extended request should not carry legacy skills")
	}
	if _, ok := requests[0].Body["persona"]; ok {
		t.Fatalf("extended request should not carry legacy persona")
	}
}

func TestNormalizeZapryAPIBaseURL(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty", in: "", want: ""},
		{name: "with slash", in: "https://openapi.mimo.immo/bot/", want: "https://openapi.mimo.immo/bot"},
		{name: "without slash", in: "https://openapi.mimo.immo/bot", want: "https://openapi.mimo.immo/bot"},
		{name: "trim spaces", in: "  https://openapi.mimo.immo/bot  ", want: "https://openapi.mimo.immo/bot"},
		{name: "trim many slashes", in: "https://openapi.mimo.immo/bot///", want: "https://openapi.mimo.immo/bot"},
	}

	for _, c := range cases {
		got := normalizeZapryAPIBaseURL(c.in)
		if got != c.want {
			t.Fatalf("%s: expected %q, got %q", c.name, c.want, got)
		}
	}
}
