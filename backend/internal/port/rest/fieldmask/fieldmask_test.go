package fieldmask

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

type response struct {
	Items         []item   `json:"items"`
	NextPageToken string   `json:"next_page_token"`
	Count         int64    `json:"count"`
	Enabled       bool     `json:"enabled"`
	Empty         []string `json:"empty"`
}

type item struct {
	ID      string  `json:"id"`
	Title   string  `json:"title"`
	Details detail  `json:"details"`
	Maybe   *string `json:"maybe"`
}

type detail struct {
	Value string `json:"value"`
	Flag  bool   `json:"flag"`
}

func TestParseAliasesNestingAndCanonicalForm(t *testing.T) {
	mask, err := Parse("nextPageToken, items(title, details.flag), items.id", response{})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := mask.Canonical(), "items(details(flag),id,title),next_page_token"; got != want {
		t.Fatalf("Canonical() = %q, want %q", got, want)
	}
	other, err := Parse("items(id,title,details(flag)),next_page_token", response{})
	if err != nil {
		t.Fatal(err)
	}
	if other.Canonical() != mask.Canonical() {
		t.Fatalf("equivalent masks differ: %q and %q", mask.Canonical(), other.Canonical())
	}
}

func TestParseWildcardAndOmittedMask(t *testing.T) {
	for _, input := range []string{"", "*"} {
		mask, err := Parse(input, response{})
		if err != nil {
			t.Fatalf("Parse(%q): %v", input, err)
		}
		if mask.Canonical() != "*" {
			t.Fatalf("Parse(%q).Canonical() = %q", input, mask.Canonical())
		}
	}
	if _, err := ParseOptional("", true, response{}); !errors.Is(err, ErrInvalidMask) {
		t.Fatalf("explicit empty mask error = %v", err)
	}
	for _, input := range []string{"items(*)", "items.*"} {
		mask, err := Parse(input, response{})
		if err != nil {
			t.Fatalf("Parse(%q): %v", input, err)
		}
		if mask.Canonical() != "items" {
			t.Fatalf("Parse(%q).Canonical() = %q, want items", input, mask.Canonical())
		}
	}
}

func TestProjectKeepsScalarRepresentationsAndProjectsNestedArrays(t *testing.T) {
	mask, err := Parse("items(id,details.flag,maybe),count,enabled,empty", response{})
	if err != nil {
		t.Fatal(err)
	}
	input := json.RawMessage(`{"items":[{"id":"a","title":"hide","details":{"value":"hide","flag":false},"maybe":null},{"id":"b","title":"hide","details":{"value":"hide","flag":true},"maybe":"x"}],"next_page_token":"hide","count":9007199254740993,"enabled":false,"empty":[],"other":1}`)
	projected, err := mask.Project(input)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"count":9007199254740993,"empty":[],"enabled":false,"items":[{"details":{"flag":false},"id":"a","maybe":null},{"details":{"flag":true},"id":"b","maybe":"x"}]}`
	if string(projected) != want {
		t.Fatalf("Project() = %s, want %s", projected, want)
	}
}

func TestProjectParentSelectsWholeNestedValue(t *testing.T) {
	mask, err := Parse("items", response{})
	if err != nil {
		t.Fatal(err)
	}
	got, err := mask.Project(response{Items: []item{{ID: "id", Title: "title", Details: detail{Value: "v"}}}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), `"title":"title"`) || strings.Contains(string(got), `"next_page_token"`) {
		t.Fatalf("Project() = %s", got)
	}
}

func TestParseRejectsMalformedUnknownAndWrongNesting(t *testing.T) {
	for _, input := range []string{
		"unknown", "items.unknown", "count.value", "count(*)", "count.*", "items(", "items()", "items(id,)", "*,items", "items(*,id)", "items(id",
	} {
		if _, err := Parse(input, response{}); !errors.Is(err, ErrInvalidMask) {
			t.Errorf("Parse(%q) error = %v, want ErrInvalidMask", input, err)
		}
	}
}

func TestParseEnforcesInputAndDepthLimits(t *testing.T) {
	if _, err := Parse(strings.Repeat("x", maxInputLength+1), response{}); !errors.Is(err, ErrInvalidMask) {
		t.Fatalf("oversized mask error = %v", err)
	}
	deep := "items"
	for i := 0; i <= maxDepth; i++ {
		deep += "(details"
	}
	deep += strings.Repeat(")", maxDepth+1)
	if _, err := Parse(deep, response{}); !errors.Is(err, ErrInvalidMask) {
		t.Fatalf("deep mask error = %v", err)
	}
}
