package lsp

import (
	"context"
	"fmt"
	sitter "github.com/smacker/go-tree-sitter"
	binding "scss-lsp/scss_binding"
)

type Parser struct {
	Parser           *sitter.Parser
	stylesheetQuery  *sitter.Query
	selectorQuery    *sitter.Query
	declerationQuery *sitter.Query
	mixinQuery       *sitter.Query
	functionQuery    *sitter.Query
}

func NewParser() *Parser {
	parser := sitter.NewParser()
	parser.SetLanguage(binding.GetLanguage())

	stylesheetQuery, err1 := sitter.NewQuery([]byte("(rule_set) @capture"), binding.GetLanguage())
	selectorQuery, err2 := sitter.NewQuery([]byte("(rule_set (selectors) @capture)"), binding.GetLanguage())
	declerationQuery, err3 := sitter.NewQuery([]byte("(declaration) @dec"), binding.GetLanguage())
	mixinQuery, err4 := sitter.NewQuery([]byte("(mixin_statement) @dec"), binding.GetLanguage())
	functionQuery, err5 := sitter.NewQuery([]byte("(function_statement) @dec"), binding.GetLanguage())

	if err1 != nil || err2 != nil || err3 != nil || err4 != nil || err5 != nil {
		fmt.Println(err1)
		fmt.Println(err2)
		fmt.Println(err3)
		fmt.Println(err4)
		fmt.Println(err5)
		panic(fmt.Sprintf("failed to create queries"))
	}

	return &Parser{
		Parser:           parser,
		stylesheetQuery:  stylesheetQuery,
		selectorQuery:    selectorQuery,
		declerationQuery: declerationQuery,
		mixinQuery:       mixinQuery,
		functionQuery:    functionQuery,
	}
}

func (p *Parser) ParseString(text string, tree *sitter.Tree) (*sitter.Tree, error) {
	// tree can be null i tihnk?
	tree, err := p.Parser.ParseCtx(context.TODO(), tree, []byte(text))
	return tree, err
}

func (p *Parser) ParseBytes(text []byte, tree *sitter.Tree) (*sitter.Tree, error) {
	// tree can be null i tihnk?
	tree, err := p.Parser.ParseCtx(context.TODO(), tree, text)
	return tree, err
}

func (p *Parser) ParseTree(parsed_tree *ParsedTree) []Entry {
	// extract class selectors from the tree
	tree := parsed_tree.Tree
	input := parsed_tree.Input
	root := tree.RootNode()
	cursor := sitter.NewQueryCursor()
	cursor.Exec(p.stylesheetQuery, root)
	// i think you can somehow find out how many rulesets there are in the tree
	// but i dont know how
	entries := make([]Entry, 0)
	for {
		match, ok := cursor.NextMatch()
		if !ok {
			break
		}
		// always rule_set node
		rule_set_node := match.Captures[0].Node
		name := p.ParseRuleSet(rule_set_node, input)
		position := rule_set_node.StartPoint()
		entries = append(entries, Entry{name: name, position: position})
	}
	return entries
}

func (p *Parser) ParseRuleSet(rule_set_node *sitter.Node, input *[]byte) string {
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
	node, has_node := cursor.NextMatch()
	if has_node == false {
		return ""
	}
	return node.Captures[0].Node.Content(*input)
}

func (p *Parser) ParseMixinsInTree(parsed_tree *ParsedTree) []OnHover {
	tree := parsed_tree.Tree
	input := parsed_tree.Input
	cursor := sitter.NewQueryCursor()
	root := tree.RootNode()
	cursor.Exec(p.mixinQuery, root)
	mixins := make([]OnHover, 0)
	for {
		match, ok := cursor.NextMatch()
		if !ok {
			break
		}
		// always mixin_statement node
		mixin_statement_node := match.Captures[0].Node

		// getfieldbyname doesnt want to work for some reason
		// this is good enough for now
		name := mixin_statement_node.NamedChild(0)
		parameters := mixin_statement_node.NamedChild(1)
		body := name.Content(*input) + parameters.Content(*input)
		position := mixin_statement_node.StartPoint()
		mixins = append(mixins, OnHover{name: name.Content(*input), body: body, position: position})
	}
	return mixins
}

func (p *Parser) ParseFunctionsInTree(parsed_tree *ParsedTree) []OnHover {
	tree := parsed_tree.Tree
	input := parsed_tree.Input
	cursor := sitter.NewQueryCursor()
	root := tree.RootNode()
	cursor.Exec(p.functionQuery, root)
	functions := make([]OnHover, 0)
	for {
		match, ok := cursor.NextMatch()
		if !ok {
			break
		}
		// always function_statement node
		function_statement_node := match.Captures[0].Node
		// getfieldbyname doesnt want to work for some reason
		// this is good enough for now
		name := function_statement_node.NamedChild(0)
		parameters := function_statement_node.NamedChild(1)
		body := name.Content(*input) + parameters.Content(*input)
		position := function_statement_node.StartPoint()
		functions = append(functions, OnHover{name: name.Content(*input), body: body, position: position})
	}
	return functions
}

func (p *Parser) ParseVariablesInTree(parsed_tree *ParsedTree) []OnHover {
	tree := parsed_tree.Tree
	input := parsed_tree.Input
	cursor := sitter.NewQueryCursor()
	root := tree.RootNode()
	cursor.Exec(p.declerationQuery, root)
	variables := make([]OnHover, 0)
	for {
		match, ok := cursor.NextMatch()
		if !ok {
			break
		}
		// always function_statement node
		declearation_node := match.Captures[0].Node
		// getfieldbyname doesnt want to work for some reason
		// this is good enough for now
		name := declearation_node.NamedChild(0)
		body := declearation_node.Content(*input)
		position := declearation_node.StartPoint()
		variables = append(variables, OnHover{name: name.Content(*input), body: body, position: position})
	}
	return variables
}
