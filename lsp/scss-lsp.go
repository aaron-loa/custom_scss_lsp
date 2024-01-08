package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	sitter "github.com/smacker/go-tree-sitter"
	rpc2 "go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"io"
	"os"
	"path/filepath"
)

type ParsedTree struct {
	// treesitter tree was made from this input, needs to be synced
	Input *[]byte
	Tree  *sitter.Tree
}

type Lsp struct {
	RootPath        string
	Parser          *Parser
	RootConn        rpc2.Conn
	SelectorEntries map[string][]Entry
	Trees           map[string]*ParsedTree
	Mixins          map[string][]OnHover
	Functions       map[string][]OnHover
	Variables       map[string][]OnHover
}

type Entry struct {
	name           string
	start_position sitter.Point
	end_position   sitter.Point
}

type OnHover struct {
	name           string
	body           string
	start_position sitter.Point
	end_position   sitter.Point
}

func DefaultLsp() *Lsp {
	return &Lsp{
		RootPath:        "",
		Parser:          NewParser(),
		Trees:           make(map[string]*ParsedTree),
		SelectorEntries: make(map[string][]Entry),
		Mixins:          make(map[string][]OnHover),
		Functions:       make(map[string][]OnHover),
		Variables:       make(map[string][]OnHover),
	}
}

func (lsp *Lsp) WalkFromRoot() {
	exclude_dirs := []string{".git", "node_modules", "build"}
	filepath.WalkDir(lsp.RootPath, func(path string, d os.DirEntry, err error) error {
		for _, e := range exclude_dirs {
			if e == d.Name() && d.IsDir() {
				return filepath.SkipDir
			}
		}
		extension := filepath.Ext(path)

		if extension == ".scss" || extension == ".css" {
			file, err := os.Open(path)
			if err != nil {
				lsp.Log(err.Error(), protocol.MessageTypeError)
				return nil
			}

			text, err := io.ReadAll(file)
			if err != nil {
				lsp.Log(err.Error(), protocol.MessageTypeError)
				return nil
			}

			tree, err := lsp.Parser.ParseBytes(text, nil)
			if err != nil {
				lsp.Log(err.Error(), protocol.MessageTypeError)
				return nil
			}

			lsp.Trees[path] = &ParsedTree{
				Tree:  tree,
				Input: &text,
			}
		}

		return nil
	})
}

func (lsp *Lsp) ParseAllTrees() {
	for path, tree := range lsp.Trees {
		lsp.UpdateTree(tree, path)
	}
}

func (lsp *Lsp) UpdateTree(tree *ParsedTree, path string) {
	lsp.SelectorEntries[path] = lsp.Parser.ParseTree(tree)
	lsp.Mixins[path] = lsp.Parser.ParseMixinsInTree(tree)
	lsp.Functions[path] = lsp.Parser.ParseFunctionsInTree(tree)
	lsp.Variables[path] = lsp.Parser.ParseVariablesInTree(tree)
}

func (lsp *Lsp) findHoverableByNameInMap(name string, in_this *map[string][]OnHover) string {
	for path, array := range *in_this {
		for _, entry := range array {
			if entry.name == name {
				return name + "\n defined in: " + path
			}
		}
	}
	return ""
}

func (lsp *Lsp) findHoverableByName(name string) string {
	hover_info := lsp.findHoverableByNameInMap(name, &lsp.Mixins)
	if hover_info != "" {
		return hover_info
	}

	hover_info = lsp.findHoverableByNameInMap(name, &lsp.Functions)
	if hover_info != "" {
		return hover_info
	}

	hover_info = lsp.findHoverableByNameInMap(name, &lsp.Variables)
	if hover_info != "" {
		return hover_info
	}

	return ""
}

func (lsp *Lsp) GetHoverInfo(path string, position sitter.Point) string {
	tree := lsp.Trees[path]
	if tree == nil {
		return ""
	}
	ts_tree := tree.Tree
	input := tree.Input
	root := ts_tree.RootNode()
	node := root.NamedDescendantForPointRange(position, position)
	if node == nil {
		return ""
	}
	return lsp.findHoverableByName(node.Content(*input)) 
}

func isPointInRange(needle sitter.Point, start_position sitter.Point, end_position sitter.Point) bool {
	return needle.Row >= start_position.Row &&
		needle.Row <= end_position.Row &&
		needle.Column >= start_position.Column &&
		needle.Column <= end_position.Column
}

func (lsp *Lsp) LspHandler(ctx context.Context, reply rpc2.Replier, req rpc2.Request) error {
	switch req.Method() {
	case protocol.MethodInitialize:
		params := req.Params()
		var replyParams protocol.InitializeParams
		err := json.Unmarshal(params, &replyParams)

		if err != nil {
			lsp.Log("cant unmarshal params", protocol.MessageTypeError)
		}

		// RootURI is deprecated? but everything uses it? hmmm
		lsp.Log(fmt.Sprintf("%+v", replyParams.RootURI.Filename()), protocol.MessageTypeError)
		if replyParams.RootURI != "" {
			lsp.RootPath = replyParams.RootURI.Filename()
		} else {
			ctx.Done()
			return reply(ctx, fmt.Errorf("no root path"), nil)
		}

		go func() {
			lsp.WalkFromRoot()
			lsp.ParseAllTrees()
		}()

		return reply(ctx, protocol.InitializeResult{
			Capabilities: protocol.ServerCapabilities{
				DefinitionProvider: true,
				HoverProvider:      true,
				TextDocumentSync: protocol.TextDocumentSyncOptions{
					Change:    protocol.TextDocumentSyncKindIncremental,
					OpenClose: false,
					Save: &protocol.SaveOptions{
						IncludeText: true,
					},
				},
			},
		}, nil)
	case protocol.MethodTextDocumentDidSave:
		params := req.Params()
		var replyParams protocol.DidSaveTextDocumentParams
		err := json.Unmarshal(params, &replyParams)
		if err != nil {
			lsp.Log(err.Error(), protocol.MessageTypeError)
			return nil
		}

		path := replyParams.TextDocument.URI.Filename()
		if lsp.Trees[path] == nil {
			lsp.Log("no_tree", protocol.MessageTypeError)
			return reply(ctx, fmt.Errorf("not my file"), nil)
		}

		input := []byte(replyParams.Text)
		// TODO figure out how to calculate the diff effectively
		// lets just update the tree and ignore the old one for now
		// calculating the diff is probably more expensive that parsing it again
		tree, err := lsp.Parser.ParseBytes(input, nil)
		if err != nil {
			lsp.Log("parse_error", protocol.MessageTypeError)
			return reply(ctx, fmt.Errorf("goodbye"), fmt.Errorf("error parsing tree of: %s", path))
		}
		lsp.Trees[path].Tree = tree
		lsp.Trees[path].Input = &input
		lsp.UpdateTree(lsp.Trees[path], path)
		return reply(ctx, nil, nil)

	case protocol.MethodTextDocumentHover:
		params := req.Params()
		var replyParams protocol.HoverParams
		err := json.Unmarshal(params, &replyParams)
		if err != nil {
			return reply(ctx, fmt.Errorf("?"), nil)
		}
		path := replyParams.TextDocument.URI.Filename()
		position := replyParams.Position
		tree_point := sitter.Point{
			Row:    position.Line,
			Column: position.Character,
		}
		hover_info := lsp.GetHoverInfo(path, tree_point)
    lsp.Log(hover_info, protocol.MessageTypeError)
    if hover_info == "" {
      return reply(ctx, fmt.Errorf("no res"), nil)
    }
		return reply(ctx, protocol.Hover{
			Contents: protocol.MarkupContent{
				Kind:  protocol.PlainText,
				Value: hover_info,
			},
			Range: &protocol.Range{},
		}, nil)
	case protocol.MethodShutdown:
		// without this pylsp-test throws an error, but it's useless, i think
		return reply(ctx, fmt.Errorf("goodbye"), nil)
	}
	// always return something otherwise other lsps responses can get ruined
	// err shows up in the client as a popup/somewhere else in the UI in neovim
	// in the statusline, result is the result of the request
	return reply(ctx, fmt.Errorf("method not found: %q", req.Method()), nil)
}

func (lsp *Lsp) Init() {
	lsp = DefaultLsp()

	bufStream := rpc2.NewStream(&rwc{os.Stdin, os.Stdout})
	lsp.RootConn = rpc2.NewConn(bufStream)

	ctx := context.Background()
	lsp.RootConn.Go(ctx, lsp.LspHandler)
	<-lsp.RootConn.Done()
}

func (lsp *Lsp) Log(message string, messageType protocol.MessageType) {
	lsp.RootConn.Notify(context.Background(), protocol.MethodWindowLogMessage, protocol.LogMessageParams{
		Message: fmt.Sprintf("SCSS-LSP: %s", message),
		Type:    messageType,
	})
}

type rwc struct {
	r io.ReadCloser
	w io.WriteCloser
}

func (rwc *rwc) Read(b []byte) (int, error)  { return rwc.r.Read(b) }
func (rwc *rwc) Write(b []byte) (int, error) { return rwc.w.Write(b) }
func (rwc *rwc) Close() error {
	rwc.r.Close()
	return rwc.w.Close()
}
