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
	expected := []Entry{
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
}

func TestMixinParse(t *testing.T) {
	lsp := DefaultLsp()

	lsp.RootPath = "/home/ron/programs/scss-lsp/test_dir"
	test_tree := "/home/ron/programs/scss-lsp/test_dir/mixins_functions.scss"

	if lsp == nil {
		t.Fatalf("failed to create lsp")
	}

	lsp.WalkFromRoot()
	local_parser := NewParser()

	entries := local_parser.ParseMixinsInTree(lsp.Trees[test_tree])
	expected := []OnHover{
		{
			name:     "test_mixin_a",
			body:     "test_mixin_a($color)",
			position: sitter.Point{},
		},
		{
			name:     "test_mixin_b",
			body:     "test_mixin_b($color, $one_more)",
			position: sitter.Point{},
		},
		{
			name:     "test_mixin_c",
			body:     "test_mixin_c()",
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
		if entries[idx].body != expected[idx].body {
			t.Fatalf("expected %s, got %s", expected[idx].body, entries[idx].body)
		}
	}
}

func TestFunctionParse(t *testing.T) {
	lsp := DefaultLsp()

	lsp.RootPath = "/home/ron/programs/scss-lsp/test_dir"
	test_tree := "/home/ron/programs/scss-lsp/test_dir/mixins_functions.scss"

	if lsp == nil {
		t.Fatalf("failed to create lsp")
	}

	lsp.WalkFromRoot()
	local_parser := NewParser()

	entries := local_parser.ParseFunctionsInTree(lsp.Trees[test_tree])
	expected := []OnHover{
		{
			name:     "test_function_a",
			body:     "test_function_a($color)",
			position: sitter.Point{},
		},
		{
			name:     "test_function_b",
			body:     "test_function_b($color, $one_more)",
			position: sitter.Point{},
		},
		{
			name:     "test_function_c",
			body:     "test_function_c()",
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
		if entries[idx].body != expected[idx].body {
			t.Fatalf("expected %s, got %s", expected[idx].body, entries[idx].body)
		}
	}
}


func TestVariablesParse(t *testing.T) {
	lsp := DefaultLsp()

	lsp.RootPath = "/home/ron/programs/scss-lsp/test_dir"
	test_tree := "/home/ron/programs/scss-lsp/test_dir/variables.scss"

	if lsp == nil {
		t.Fatalf("failed to create lsp")
	}

	lsp.WalkFromRoot()
	local_parser := NewParser()

	entries := local_parser.ParseVariablesInTree(lsp.Trees[test_tree])
	expected := []OnHover{
    {
      name: "$color1",
      body: "$color1: #000;",
      position: sitter.Point{},
    },
    {
      name: "$color2",
      body: "$color2: #100;",
      position: sitter.Point{},
    },
    {
      name: "$color3",
      body: "$color3: #200;",
      position: sitter.Point{},
    },
    {
      name: "$color4",
      body: "$color4: #300;",
      position: sitter.Point{},
    },
    {
      name: "$color5",
      body: "$color5: #400;",
      position: sitter.Point{},
    },
    {
      name: "$color6",
      body: "$color6: #500;",
      position: sitter.Point{},
    },
    {
      name: "$color7",
      body: "$color7: #600;",
      position: sitter.Point{},
    },
    {
      name: "$color8",
      body: "$color8: #700;",
      position: sitter.Point{},
    },
    {
      name: "$function_return",
      body: "$function_return: floor(1);",
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
		if entries[idx].body != expected[idx].body {
			t.Fatalf("expected %s, got %s", expected[idx].body, entries[idx].body)
		}
	}
}
