package lsp

import (
	"context"
	"fmt"
	sitter "github.com/smacker/go-tree-sitter"
	binding "scss-lsp/scss_binding"
)

type Parser struct {
	Parser          *sitter.Parser
	stylesheetQuery *sitter.Query
	selectorQuery   *sitter.Query
}

func NewParser() *Parser {
	parser := sitter.NewParser()
	parser.SetLanguage(binding.GetLanguage())

	stylesheetQuery, err1 := sitter.NewQuery([]byte("(rule_set) @capture"), binding.GetLanguage())
	selectorQuery, err2 := sitter.NewQuery([]byte("(rule_set (selectors) @capture)"), binding.GetLanguage())

	if err1 != nil || err2 != nil  {
		fmt.Println(err1)
		fmt.Println(err2)
		panic(fmt.Sprintf("failed to create queries"))
	}
	return &Parser{
		Parser:          parser,
		stylesheetQuery: stylesheetQuery,
		selectorQuery:   selectorQuery,
	}
}

func (p *Parser) ParseString(text string, tree *sitter.Tree) (*sitter.Tree, error) {
	// old_tree can be null i tihnk?
	tree, err := p.Parser.ParseCtx(context.TODO(), tree, []byte(text))
	return tree, err
}

func (p *Parser) ParseBytes(text []byte, tree *sitter.Tree) (*sitter.Tree, error) {
	// old_tree can be null i tihnk?
	tree, err := p.Parser.ParseCtx(context.TODO(), tree, text)
	return tree, err
}

func (p *Parser) ParseTree(parsed_tree *ParsedTree) []SelectorEntry {
	// extract class selectors from the tree
	tree := parsed_tree.Tree
	input := parsed_tree.Input
	root := tree.RootNode()
	cursor := sitter.NewQueryCursor()
	cursor.Exec(p.stylesheetQuery, root)
	// i think you can somehow find out how many rulesets there are in the tree
	// but i dont know how
	entries := make([]SelectorEntry, 0)
	for {
		match, ok := cursor.NextMatch()
		if !ok {
			break
		}
		// always rule_set node
		rule_set_node := match.Captures[0].Node
		name := p.ParseRuleSet(rule_set_node, input)
		position := rule_set_node.StartPoint()
		entries = append(entries, SelectorEntry{name: name, position: position})
	}
	return entries
}

func (p *Parser) ParseRuleSet(rule_set_node *sitter.Node, input *[]byte) string {
	// TODO: parse ruleset
	// parse the current ruleset, and then call parse ruleset on the children
	// rulesets
	// has to return the selector of the ruleset, and it needs
	// to concat these together to get the full selector
	// also needs to get the position of the ruleset
	// with this selector we call this function again, and push to this string
	// the next selector
	// this should work
	name := p.ParseSelectors(rule_set_node, input)
	parent := rule_set_node.Parent()
	for {
		if parent == nil {
			break
		}
		if parent.Type() == "rule_set" {
			name = p.ParseSelectors(parent, input) + " " + name
		}
		parent = parent.Parent()
	}
	return name
}

func (p *Parser) ParseSelectors(rule_set_node *sitter.Node, input *[]byte) string {
	cursor := sitter.NewQueryCursor()
	cursor.Exec(p.selectorQuery, rule_set_node)
	node, is_empty := cursor.NextMatch()
	if is_empty == false {
		return ""
	}
	return node.Captures[0].Node.Content(*input)

}

func (p *Parser) ParseMixinsInTree( /* ??? */ ) {
	// TODO: parse mixin
	// just get the mixin name, nothing complicated, return it as string+position
}
