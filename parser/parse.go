package parser

import (
	"context"
	binding "scss-lsp/scss_binding"
	sitter "github.com/smacker/go-tree-sitter"
)

type Parser struct {
	Parser *sitter.Parser
}

func NewParser() *Parser {
	parser := sitter.NewParser()
	parser.SetLanguage(binding.GetLanguage())

	return &Parser{
		Parser: parser,
	}
}

func (p *Parser) ParseString(text string, old_tree *sitter.Tree) (*sitter.Tree, error) {
	// old_tree can be null i tihnk?
	tree, err := p.Parser.ParseCtx(context.TODO(), old_tree, []byte(text))
	return tree, err
}
func (p *Parser) ParseBytes(text []byte, old_tree *sitter.Tree) (*sitter.Tree, error) {
	// old_tree can be null i tihnk?
	tree, err := p.Parser.ParseCtx(context.TODO(), old_tree, text)
	return tree, err
}
