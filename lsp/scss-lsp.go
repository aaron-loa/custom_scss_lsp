package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	parser "scss-lsp/parser"

	sitter "github.com/smacker/go-tree-sitter"
	rpc2 "go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

type Lsp struct {
	rootPath string
	parser   *parser.Parser
	trees    map[string]*sitter.Tree
	rootConn rpc2.Conn
}

func (lsp *Lsp) WalkFromRoot() {
	exclude_dirs := []string{".git", "node_modules", "build"}
	filepath.WalkDir(lsp.rootPath, func(path string, d os.DirEntry, err error) error {
		for _, e := range exclude_dirs {
			if e == d.Name() && d.IsDir() {
				return filepath.SkipDir
			}
		}
		extension := filepath.Ext(path)
		fmt.Println(extension)
    
		if extension == ".scss" || extension == ".css" {
			file, err := os.Open(path)
			if err != nil {
				lsp.Log(err.Error(), protocol.MessageTypeError)
			}

			text, err := io.ReadAll(file)
			if err != nil {
				lsp.Log(err.Error(), protocol.MessageTypeError)
			}

			tree, err := lsp.parser.ParseBytes(text, nil)
			if err != nil {
				lsp.Log(err.Error(), protocol.MessageTypeError)
			}
			lsp.trees[path] = tree
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
		lsp.Log(fmt.Sprintf("%+v", replyParams.RootURI), protocol.MessageTypeError)

		if replyParams.RootURI != "" {
			lsp.rootPath = replyParams.RootURI.Filename()
		} else {
			ctx.Done()
			return reply(ctx, fmt.Errorf("no root path"), nil)
		}

		return reply(ctx, protocol.InitializeResult{
			Capabilities: protocol.ServerCapabilities{
				DefinitionProvider: true,
				HoverProvider:      true,
				TextDocumentSync: protocol.TextDocumentSyncOptions{
					Change:    protocol.TextDocumentSyncKindFull,
					OpenClose: true,
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
func DefaultLsp() *Lsp {
  return &Lsp{
    rootPath: "", 
    parser: parser.NewParser(),
    trees: make(map[string]*sitter.Tree),
  }
}
func (lsp *Lsp) Init() {
  lsp = DefaultLsp()

	bufStream := rpc2.NewStream(&rwc{os.Stdin, os.Stdout})
	lsp.rootConn = rpc2.NewConn(bufStream)

	ctx := context.Background()
	lsp.rootConn.Go(ctx, lsp.LspHandler)
	<-lsp.rootConn.Done()
}

func (lsp *Lsp) Log(message string, messageType protocol.MessageType) {
	lsp.rootConn.Notify(context.Background(), protocol.MethodWindowLogMessage, protocol.LogMessageParams{
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
