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
		name := p.parseRuleSet(rule_set_node, input)
		start_position := rule_set_node.StartPoint()
		end_position := rule_set_node.StartPoint()
		entries = append(entries, Entry{name: name, start_position: start_position, end_position: end_position})
	}
	return entries
}

func (p *Parser) parseRuleSet(rule_set_node *sitter.Node, input *[]byte) string {
	name := p.parseSelectors(rule_set_node, input)
	parent := rule_set_node.Parent()
	for {
		if parent == nil {
			break
		}
		if parent.Type() == "rule_set" {
			name = p.parseSelectors(parent, input) + " " + name
		}
		parent = parent.Parent()
	}
	return name
}

func (p *Parser) parseSelectors(rule_set_node *sitter.Node, input *[]byte) string {
	cursor := sitter.NewQueryCursor()
	cursor.Exec(p.selectorQuery, rule_set_node)
	node, has_node := cursor.NextMatch()
	if has_node == false {
		return ""
	}
	return node.Captures[0].Node.Content(*input)
}

func (p *Parser) ParseMixinsInTree(parsed_tree *ParsedTree) []isDefined {
	tree := parsed_tree.Tree
	input := parsed_tree.Input
	cursor := sitter.NewQueryCursor()
	root := tree.RootNode()
	cursor.Exec(p.mixinQuery, root)
	mixins := make([]isDefined, 0)
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
		start_position := mixin_statement_node.StartPoint()
		end_position := mixin_statement_node.EndPoint()
		mixins = append(mixins, isDefined{name: name.Content(*input), body: body, start_position: start_position, end_position: end_position})
	}
	return mixins
}

func (p *Parser) ParseFunctionsInTree(parsed_tree *ParsedTree) []isDefined {
	tree := parsed_tree.Tree
	input := parsed_tree.Input
	cursor := sitter.NewQueryCursor()
	root := tree.RootNode()
	cursor.Exec(p.functionQuery, root)
	functions := make([]isDefined, 0)
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
		start_position := function_statement_node.StartPoint()
		end_position := function_statement_node.EndPoint()
		functions = append(functions, isDefined{name: name.Content(*input), body: body, start_position: start_position, end_position: end_position})
	}
	return functions
}

func (p *Parser) ParseVariablesInTree(parsed_tree *ParsedTree) []isDefined {
	tree := parsed_tree.Tree
	input := parsed_tree.Input
	cursor := sitter.NewQueryCursor()
	root := tree.RootNode()
	cursor.Exec(p.declerationQuery, root)
	variables := make([]isDefined, 0)
	for {
		match, ok := cursor.NextMatch()
		if !ok {
			break
		}
		// always function_statement node
		declaration_node := match.Captures[0].Node
		// this doesnt want to work for some reason, probably doing something wrong
		// name := declearation_node.ChildByFieldName("name")
		name := declaration_node.NamedChild(0)
		body := declaration_node.Content(*input)
		start_position := declaration_node.StartPoint()
		end_position := declaration_node.EndPoint()
		variables = append(variables, isDefined{name: name.Content(*input), body: body, start_position: start_position, end_position: end_position})
	}
	return variables
}
