package prompts

import (
	"strings"
	"testing"
	"time"

	"github.com/CamiloValderruten/faultline/internal/search/bm25"
	"github.com/CamiloValderruten/faultline/internal/skills"
	"github.com/CamiloValderruten/faultline/internal/subagent"
)

func TestBuildCycleContext_NoMemories(t *testing.T) {
	now := time.Date(2026, 4, 27, 10, 30, 0, 0, time.UTC)
	got := BuildCycleContext("SYSTEM PROMPT", nil, nil, nil, "", now, 2000)

	if !strings.Contains(got, "SYSTEM PROMPT") {
		t.Error("output missing system prompt")
	}
	if !strings.Contains(got, now.Format(time.RFC1123)) {
		t.Error("output missing current time")
	}
	if strings.Contains(got, "Recent Memories") {
		t.Error("output should not have Recent Memories section when no memories provided")
	}
	if strings.Contains(got, "Available Skills") {
		t.Error("output should not have Available Skills section when no skills provided")
	}
	if strings.Contains(got, "Subagent Profiles") {
		t.Error("output should not have Subagent Profiles section when no profiles provided")
	}
}

func TestBuildCycleContext_WithMemories(t *testing.T) {
	now := time.Now()
	mems := []bm25.Result{
		{Path: "alpha.md", Content: "alpha content"},
		{Path: "beta.md", Content: "beta content"},
	}
	got := BuildCycleContext("SYS", mems, nil, nil, "", now, 2000)

	if !strings.Contains(got, "Recent Memories") {
		t.Error("missing Recent Memories header")
	}
	if !strings.Contains(got, "### alpha.md") {
		t.Error("missing alpha header")
	}
	if !strings.Contains(got, "### beta.md") {
		t.Error("missing beta header")
	}
	if !strings.Contains(got, "alpha content") {
		t.Error("missing alpha body")
	}
}

func TestBuildCycleContext_TruncatesLongMemory(t *testing.T) {
	long := strings.Repeat("x", 3000)
	mems := []bm25.Result{{Path: "long.md", Content: long}}
	got := BuildCycleContext("SYS", mems, nil, nil, "", time.Now(), 2000)

	if !strings.Contains(got, "[truncated") {
		t.Error("expected truncation marker for long memory")
	}
	// Body should not contain the full 3000 x's
	if strings.Count(got, "x") >= 3000 {
		t.Errorf("memory was not truncated; got %d x's", strings.Count(got, "x"))
	}
	// Hint must mention the tool the agent should call to read the rest.
	if !strings.Contains(got, "memory_read") {
		t.Error("expected truncation hint to reference memory_read")
	}
	// Hint must mention the file path so the agent doesn't have to guess.
	if !strings.Contains(got, "long.md") {
		t.Error("expected truncation hint to reference the file path")
	}
}

func TestBuildCycleContext_NoLimitKeepsFullContent(t *testing.T) {
	long := strings.Repeat("x", 3000)
	mems := []bm25.Result{{Path: "long.md", Content: long}}
	got := BuildCycleContext("SYS", mems, nil, nil, "", time.Now(), 0)

	if strings.Contains(got, "[truncated") {
		t.Error("did not expect truncation marker when limit is disabled")
	}
	if strings.Count(got, "x") < 3000 {
		t.Errorf("expected full 3000 x's when limit disabled; got %d", strings.Count(got, "x"))
	}
}

func TestBuildCycleContext_WithSkills(t *testing.T) {
	cat := []skills.Skill{
		{Name: "pdf-processing", Description: "Handle PDFs.", Source: skills.SourceUser},
		{Name: "data-analysis", Description: "Analyze datasets.", Source: skills.SourceUser},
	}
	got := BuildCycleContext("SYS", nil, cat, nil, "", time.Now(), 2000)

	if !strings.Contains(got, "## Available Skills") {
		t.Error("missing Available Skills header")
	}
	if !strings.Contains(got, "### User") {
		t.Error("missing User subsection")
	}
	if !strings.Contains(got, "**pdf-processing**: Handle PDFs.") {
		t.Errorf("missing pdf-processing entry; got %q", got)
	}
	if !strings.Contains(got, "**data-analysis**: Analyze datasets.") {
		t.Error("missing data-analysis entry")
	}
	if !strings.Contains(got, "skill_activate") {
		t.Error("missing skill_activate instruction")
	}
}

func TestBuildCycleContext_SkillsSystemAndUser(t *testing.T) {
	cat := []skills.Skill{
		{Name: "html-artifact", Description: "Publish pages.", Source: skills.SourceSystem},
		{Name: "finances", Description: "Budget Q&A.", Source: skills.SourceUser},
	}
	got := BuildCycleContext("SYS", nil, cat, nil, "", time.Now(), 2000)
	sysIdx := strings.Index(got, "### System")
	userIdx := strings.Index(got, "### User")
	if sysIdx < 0 || userIdx < 0 {
		t.Fatalf("missing subsections: system=%d user=%d\n%s", sysIdx, userIdx, got)
	}
	if sysIdx > userIdx {
		t.Error("System subsection should appear before User")
	}
	if !strings.Contains(got, "**html-artifact**: Publish pages.") {
		t.Error("missing system skill entry")
	}
	if !strings.Contains(got, "**finances**: Budget Q&A.") {
		t.Error("missing user skill entry")
	}
}

func TestBuildCycleContext_SkillsAndMemoriesTogether(t *testing.T) {
	cat := []skills.Skill{{Name: "x", Description: "x."}}
	mems := []bm25.Result{{Path: "m.md", Content: "memory body"}}
	got := BuildCycleContext("SYS", mems, cat, nil, "", time.Now(), 2000)

	skillsIdx := strings.Index(got, "## Available Skills")
	memIdx := strings.Index(got, "## Recent Memories")
	if skillsIdx < 0 || memIdx < 0 {
		t.Fatalf("missing one or both sections: skillsIdx=%d memIdx=%d", skillsIdx, memIdx)
	}
	if skillsIdx > memIdx {
		t.Error("skills section should appear before memories")
	}
}

func TestBuildCycleContext_WithSubagents(t *testing.T) {
	cat := []subagent.Catalog{
		{Name: "fast", Purpose: "Quick lookups."},
		{Name: "deep", Purpose: "Architecture analysis."},
	}
	got := BuildCycleContext("SYS", nil, nil, cat, "", time.Now(), 2000)

	if !strings.Contains(got, "## Available Subagent Profiles") {
		t.Error("missing Subagent Profiles header")
	}
	if !strings.Contains(got, "**fast**: Quick lookups.") {
		t.Errorf("missing fast entry; got %q", got)
	}
	if !strings.Contains(got, "**deep**: Architecture analysis.") {
		t.Error("missing deep entry")
	}
	if !strings.Contains(got, "subagent_run") {
		t.Error("missing subagent_run instruction")
	}
}

func TestBuildCycleContext_SubagentsAfterSkills(t *testing.T) {
	skl := []skills.Skill{{Name: "x", Description: "x."}}
	sub := []subagent.Catalog{{Name: "fast", Purpose: "y"}}
	mems := []bm25.Result{{Path: "m.md", Content: "memory body"}}
	got := BuildCycleContext("SYS", mems, skl, sub, "", time.Now(), 2000)

	skillsIdx := strings.Index(got, "## Available Skills")
	subIdx := strings.Index(got, "## Available Subagent Profiles")
	memIdx := strings.Index(got, "## Recent Memories")
	if skillsIdx < 0 || subIdx < 0 || memIdx < 0 {
		t.Fatalf("missing sections: skills=%d sub=%d mem=%d", skillsIdx, subIdx, memIdx)
	}
	if skillsIdx >= subIdx || subIdx >= memIdx {
		t.Errorf("expected order skills -> subagent -> memories; got %d, %d, %d", skillsIdx, subIdx, memIdx)
	}
}

func TestBuildCycleContext_WithCollaboratorGuide(t *testing.T) {
	guide := "## Collaborator Channel\n\nYou are on Discord.\n"
	got := BuildCycleContext("SYS", nil, nil, nil, guide, time.Now(), 2000)
	if !strings.Contains(got, "You are on Discord.") {
		t.Fatal("missing collaborator guide body")
	}
	timeIdx := strings.Index(got, "**Current Time**")
	guideIdx := strings.Index(got, "## Collaborator Channel")
	if timeIdx < 0 || guideIdx < 0 || guideIdx < timeIdx {
		t.Fatalf("expected guide after current time; time=%d guide=%d", timeIdx, guideIdx)
	}
}

func TestBuildCycleContext_EmptyCollaboratorGuideOmitted(t *testing.T) {
	got := BuildCycleContext("SYS", nil, nil, nil, "   ", time.Now(), 2000)
	if strings.Contains(got, "Collaborator Channel") {
		t.Fatal("blank guide should not render a section")
	}
}
