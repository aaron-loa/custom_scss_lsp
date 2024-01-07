package lsp

import (
	sitter "github.com/smacker/go-tree-sitter"
	"testing"
)

func TestTreeParse(t *testing.T) {
	lsp := DefaultLsp()

	lsp.RootPath = "/home/ron/programs/scss-lsp/test_dir"
	test_tree := "/home/ron/programs/scss-lsp/test_dir/file_b.scss"

	if lsp == nil {
		t.Fatalf("failed to create lsp")
	}
	lsp.WalkFromRoot()
	local_parser := NewParser()
	entries := local_parser.ParseTree(lsp.Trees[test_tree])
	// TODO MAKE TEST FOR POSITIONS
	expected := []SelectorEntry{
		{
			name:     ("body .foo"),
			position: sitter.Point{},
		},
		{
			name:     ("body .foo .bar"),
			position: sitter.Point{},
		},
		{
			name:     (".level-one"),
			position: sitter.Point{},
		},
		{
			name:     (".level-one .level-two-a"),
			position: sitter.Point{},
		},
		{
			name:     (".level-one .level-two-a .level-three-a"),
			position: sitter.Point{},
		},
		{
			name:     (".level-one .level-two-a .level-three-b"),
			position: sitter.Point{},
		},
		{
			name:     (".level-one &>.level-two-b"),
			position: sitter.Point{},
		},
		{
			name:     (".another"),
			position: sitter.Point{},
		},
		{
			name:     (".another .nested"),
			position: sitter.Point{},
		},
	}
	if len(entries) != len(expected) {
		t.Fatalf("expected %d entries, got %d", len(expected), len(entries))
	}
	for idx := range entries {
		if entries[idx].name != expected[idx].name {
			t.Fatalf("expected %s, got %s", expected[idx].name, entries[idx].name)
		}
	}
  t.Fatalf("TeST")
}
