package lsp_test

import (
	"testing"
	"go.lsp.dev/uri"
  lsp "scss-lsp/lsp"
)

func makeTestLsp() *lsp.Lsp {
	local_lsp := lsp.DefaultLsp()
	parsed_uri, err := uri.Parse("file:///home/ron/programs/scss-lsp/test_dir")
	if err != nil {
		return nil
	}
	local_lsp.RootPath = parsed_uri.Filename()
	return local_lsp
}

func TestTreeWalking(t *testing.T) {
	local_lsp := makeTestLsp()
	if local_lsp == nil {
		t.Fatalf("failed to create lsp")
	}
  local_lsp.WalkFromRoot()
	// this is not great, but it is what it is
	expected := 5
	if len(local_lsp.Trees) != expected {
		t.Fatalf("expected %d trees, got %d", expected, len(local_lsp.Trees))
	}
}
