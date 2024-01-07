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
}

type Entry struct {
	name     string
	position sitter.Point
}

type OnHover struct {
	name     string
	body     string
	position sitter.Point
}

func DefaultLsp() *Lsp {
	return &Lsp{
		RootPath:        "",
		Parser:          NewParser(),
		Trees:           make(map[string]*ParsedTree),
		SelectorEntries: make(map[string][]Entry),
		Mixins:          make(map[string][]OnHover),
		Functions:       make(map[string][]OnHover),
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

		return reply(ctx, protocol.InitializeResult{
			Capabilities: protocol.ServerCapabilities{
				DefinitionProvider: true,
				HoverProvider:      true,
				TextDocumentSync: protocol.TextDocumentSyncOptions{
					OpenClose: false,
					Save: &protocol.SaveOptions{
						IncludeText: true,
					},
				},
			},
		}, nil)
		// without this pylsp-test throws an error, but it's useless, i think
	case protocol.MethodShutdown:
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
