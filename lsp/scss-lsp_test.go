package lsp

import (
	"testing"
)

func TestTreeParse(t *testing.T) {
	lsp := DefaultLsp()
	lsp.rootPath = "/home/ron/programs/scss-lsp/test_dir/"
	lsp.WalkFromRoot()
  // this is not great, but it is what it is
	expected := 3
	if len(lsp.trees) != expected {
		t.Fatalf("expected %d trees, got %d", expected, len(lsp.trees))
	}
}
