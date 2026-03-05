package agentsdk

import "testing"

func TestAgentBuilder_BuiltinSkill(t *testing.T) {
	cfg, err := NewAgentBuilder("builtin-agent", "Builtin Agent").
		BuiltinSkill("image-generation").
		Tool("render_image", "渲染图像", dummyHandler).
		Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	if cfg.Card.Capabilities == nil || len(cfg.Card.Capabilities.Skills) == 0 {
		t.Fatalf("expected builtin skill to be added")
	}
	got := cfg.Card.Capabilities.Skills[0]
	if got.Name != "image-generation" {
		t.Fatalf("expected skill name image-generation, got %s", got.Name)
	}
	if len(got.Tags) == 0 {
		t.Fatalf("expected builtin tags to be set")
	}
}

func TestAgentBuilder_BuiltinSkillsBatch(t *testing.T) {
	cfg, err := NewAgentBuilder("builtin-batch", "Builtin Batch").
		BuiltinSkills("image-generation", "knowledge-qa").
		Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	if cfg.Card.Capabilities == nil {
		t.Fatalf("expected capabilities")
	}
	if len(cfg.Card.Capabilities.Skills) != 2 {
		t.Fatalf("expected 2 builtin skills, got %d", len(cfg.Card.Capabilities.Skills))
	}
}

