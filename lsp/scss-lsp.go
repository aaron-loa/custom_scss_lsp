package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	rpc2 "go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

type Lsp struct {
	RootPath        string
	Parser          *Parser
	RootConn        rpc2.Conn
	SelectorEntries map[string][]Entry
	Trees           map[string]*sitter.Tree
	Cache           map[string][]byte
	// this was not a good idea
	// the proper way to do this is to have a map of maps, that goes like
	// path -> name -> data
	// instead of 3 maps just have 1
	// oh well, this is just a learning experience anyway
	// it made so much sense at first!
	Mixins        map[string][]isDefined
	Functions     map[string][]isDefined
	Variables     map[string][]isDefined
	Calls         map[string][]isDefined
	CallWhitelist []string
}

type Entry struct {
	name           string
	start_position sitter.Point
	end_position   sitter.Point
}

type isDefined struct {
	name           string
	body           string
	start_position sitter.Point
	end_position   sitter.Point
}

func DefaultLsp() *Lsp {
	return &Lsp{
		RootPath:        "",
		Parser:          NewParser(),
		Trees:           make(map[string]*sitter.Tree),
		SelectorEntries: make(map[string][]Entry),
		Mixins:          make(map[string][]isDefined),
		Functions:       make(map[string][]isDefined),
		Variables:       make(map[string][]isDefined),
		Calls:           make(map[string][]isDefined),
		Cache:           make(map[string][]byte),
		CallWhitelist:   []string{
      "url",
      "var",
      "translateY",
      "translateX",
      "calc",
      "linear-gradient",
      "linear-gradient",
      "repeat",
      "nth-child",
    },
	}
}

func (lsp *Lsp) getWordAtPosition(data *string, line, column int) (string, error) {
	// Convert byte array to string
	// Split the content into lines
	lines := strings.Split(*data, "\n")

	// Check if the specified line is within the range
	if line < 0 || line >= len(lines) {
		return "", fmt.Errorf("invalid line number: %d", line)
	}

	// Get the specified line
	targetLine := lines[line]
	// Check if the specified column is within the range
	if column < 0 || column >= len(targetLine) {
		return "", fmt.Errorf("invalid column number: %d", column)
	}

	// Use column information to find the word
	startIndex := column
	for startIndex > 0 && !isSeparator(targetLine[startIndex-1]) {
		startIndex--
	}

	endIndex := column
	for endIndex < len(targetLine)-1 && !isSeparator(targetLine[endIndex+1]) {
		endIndex++
	}

	// Extract the word
	word := targetLine[startIndex : endIndex+1]
	return word, nil
}

func isSeparator(char byte) bool {
	// Customize this function based on what you consider as word separators
	// For example, you might want to include characters like '.', ',', ';', etc.
	return char == ' ' ||
		char == '\t' ||
		char == '\n' ||
		char == '\r' ||
		char == '@' ||
		char == ';'
}

func (lsp *Lsp) WalkFromRoot() {
	// exclude_dirs := []string{".git", "node_modules", "build", "vendor"}
	// lets try to not ignore node_modules
	// just tested it, and it is still super fast
	exclude_dirs := []string{".git", "build", "vendor", "contrib"}
	filepath.WalkDir(lsp.RootPath, func(path string, d os.DirEntry, err error) error {
		for _, e := range exclude_dirs {
			if e == d.Name() && d.IsDir() {
				return filepath.SkipDir
			}
		}
		extension := filepath.Ext(path)

		if extension == ".scss" {
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

			tree, err := lsp.Parser.ParseBytes(&text, nil)
			if err != nil {
				lsp.Log(err.Error(), protocol.MessageTypeError)
				return nil
			}
			lsp.Trees[path] = tree
			if lsp.Trees[path] == nil {
				lsp.Trees[path] = &sitter.Tree{}
			}
			lsp.UpdateTree(lsp.Trees[path], path, &text)
		}

		return nil
	})
}
func (lsp *Lsp) ParseAndSaveTree(path string) (*sitter.Tree, error) {
	file, err := os.Open(path)
	if err != nil {
		lsp.Log(err.Error(), protocol.MessageTypeError)
		return nil, err
	}
	text, err := io.ReadAll(file)
	if err != nil {
		lsp.Log(err.Error(), protocol.MessageTypeError)
		return nil, err
	}
	tree, err := lsp.Parser.ParseBytes(&text, nil)
	if err != nil {
		lsp.Log(err.Error(), protocol.MessageTypeError)
		return nil, err
	}
	lsp.Trees[path] = tree
	lsp.UpdateTree(lsp.Trees[path], path, &text)
	return lsp.Trees[path], nil
}

func (lsp *Lsp) UpdateTree(tree *sitter.Tree, path string, input *[]byte) {
	// this doesnt work great if there are more lsps
	// so i need to figure out how to turn off specific capabilities of other lsps
	lsp.SelectorEntries[path] = lsp.Parser.ParseTree(tree, input)
	lsp.Mixins[path] = lsp.Parser.ParseMixinsInTree(tree, input)
	lsp.Functions[path] = lsp.Parser.ParseFunctionsInTree(tree, input)
	lsp.Variables[path] = lsp.Parser.ParseVariablesInTree(tree, input)
	lsp.Calls[path] = lsp.Parser.ParseCalls(tree, input)
}

func (lsp *Lsp) findHoverableByNameInMap(name *string, in_this *map[string][]isDefined, item_type *string) *[]isDefinedInfo {
	defined_info_array := []isDefinedInfo{}
	for path, array := range *in_this {
		for _, entry := range array {
			if entry.name == *name {
				info := isDefinedInfo{path: path, is_defined: entry, item_type: *item_type}
				defined_info_array = append(defined_info_array, info)
			}
		}
	}
	return &defined_info_array
}

type isDefinedInfo struct {
	path       string
	item_type  string
	is_defined isDefined
}

func (lsp *Lsp) findHoverableByName(name *string) *[]isDefinedInfo {
	defined_array := []isDefinedInfo{}

	mixin := "@mixin"
	function := "@function"
	variable := "$variable"

	is_defined_object := lsp.findHoverableByNameInMap(name, &lsp.Mixins, &mixin)
	defined_array = append(defined_array, *is_defined_object...)

	is_defined_object = lsp.findHoverableByNameInMap(name, &lsp.Functions, &function)
	defined_array = append(defined_array, *is_defined_object...)

	is_defined_object = lsp.findHoverableByNameInMap(name, &lsp.Variables, &variable)
	defined_array = append(defined_array, *is_defined_object...)

	return &defined_array
}

func (lsp *Lsp) stringFromFilePath(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		lsp.Log(err.Error(), protocol.MessageTypeError)
		return "", err
	}

	bytes, err := io.ReadAll(file)

	if err != nil {
		lsp.Log(err.Error(), protocol.MessageTypeError)
		return "", err
	}
	string := string(bytes)

	return string, nil
}

func (lsp *Lsp) bytesFromFilePath(path string) (*[]byte, error) {
	if lsp.Cache[path] != nil {
		bytes := lsp.Cache[path]
		return &bytes, nil
	}
	file, err := os.Open(path)
	if err != nil {
		lsp.Log(err.Error(), protocol.MessageTypeError)
		return nil, err
	}

	bytes, err := io.ReadAll(file)

	if err != nil {
		lsp.Log(err.Error(), protocol.MessageTypeError)
		return nil, err
	}
	return &bytes, nil
}

func (lsp *Lsp) GetHoverInfo(path string, position sitter.Point) string {
	tree := lsp.Trees[path]
	if tree == nil {
		return ""
	}
	bytes, err := lsp.bytesFromFilePath(path)
	if err != nil {
		lsp.Log(err.Error(), protocol.MessageTypeError)
		return ""
	}
	root := tree.RootNode()
	node := root.NamedDescendantForPointRange(position, position)
	input := node.Content(*bytes)
	definitions := *lsp.findHoverableByName(&input)

	if len(definitions) == 0 {
		return ""
	}
	var sb strings.Builder
	// no sass parser for markdown? unlucky
	// probably can make a neovim plugin maybe?, we just need to use sass parser
	// in the sass part of markdown hmm
	sb.WriteString("```css\n")
	sb.WriteString(definitions[0].is_defined.body)
	sb.WriteString("\n```")
	sb.WriteString("\n")
	sb.WriteString(definitions[0].item_type)
	sb.WriteString(" defined in: ")
	sb.WriteString(definitions[0].path)
	return sb.String()
}

func (lsp *Lsp) UpdateTreeBytes(path string, input *[]byte) (*sitter.Tree, error) {
	// TODO figure out how to calculate the diff effectively
	// lets just update the tree and ignore the old one for now
	// calculating the diff is probably more expensive that parsing it again
	tree, err := lsp.Parser.ParseBytes(input, nil)
	if err != nil {
		lsp.Log("parse_error", protocol.MessageTypeError)
		return nil, err
	}
	lsp.Trees[path] = tree
	lsp.UpdateTree(lsp.Trees[path], path, input)
	return lsp.Trees[path], nil
}

func (lsp *Lsp) GetDefinitionInfo(path string, position sitter.Point) *[]protocol.Location {
	tree := lsp.Trees[path]
	if tree == nil {
		return nil
	}
	ts_tree := tree
	root := ts_tree.RootNode()
	node := root.NamedDescendantForPointRange(position, position)
	if node == nil {
		return nil
	}
	input, err := lsp.bytesFromFilePath(path)
	if err != nil {
		return nil
	}
	word := node.Content(*input)
	definitions := *lsp.findHoverableByName(&word)
	if len(definitions) == 0 {
		return nil
	}

	locations := []protocol.Location{}

	for _, entry := range definitions {
		location := protocol.Location{
			URI: uri.URI("file://" + entry.path),
			Range: protocol.Range{
				Start: protocol.Position{
					Line:      entry.is_defined.start_position.Row,
					Character: entry.is_defined.start_position.Column,
				},
				End: protocol.Position{
					Line:      entry.is_defined.end_position.Row,
					Character: entry.is_defined.end_position.Column,
				},
			},
		}
		locations = append(locations, location)
	}
	return &locations
}

func isPointInRange(needle sitter.Point, start_position sitter.Point, end_position sitter.Point) bool {
	return needle.Row >= start_position.Row &&
		needle.Row <= end_position.Row &&
		needle.Column >= start_position.Column &&
		needle.Column <= end_position.Column
}

func (lsp *Lsp) gatherSymbolsInPath(path string) []protocol.SymbolInformation {
	items := make([]protocol.SymbolInformation, 0)

	for _, entry := range lsp.Mixins[path] {
		items = append(items, protocol.SymbolInformation{
			Name: entry.name,
			Kind: protocol.SymbolKindInterface,
			Location: protocol.Location{
				URI: uri.URI("file://" + path),
				Range: protocol.Range{
					Start: protocol.Position{
						Line:      entry.start_position.Row,
						Character: entry.start_position.Column,
					},
					End: protocol.Position{
						Line:      entry.start_position.Row,
						Character: entry.start_position.Column,
					},
				},
			},
		})
	}

	for _, entry := range lsp.Variables[path] {
		items = append(items, protocol.SymbolInformation{
			Name: entry.name,
			Kind: protocol.SymbolKindVariable,
			Location: protocol.Location{
				URI: uri.URI("file://" + path),
				Range: protocol.Range{
					Start: protocol.Position{
						Line:      entry.start_position.Row,
						Character: entry.start_position.Column,
					},
					End: protocol.Position{
						Line:      entry.start_position.Row,
						Character: entry.start_position.Column,
					},
				},
			},
		})
	}

	for _, entry := range lsp.Functions[path] {
		items = append(items, protocol.SymbolInformation{
			Name: entry.name,
			Kind: protocol.SymbolKindFunction,
			Location: protocol.Location{
				URI: uri.URI("file://" + path),
				Range: protocol.Range{
					Start: protocol.Position{
						Line:      entry.start_position.Row,
						Character: entry.start_position.Column,
					},
					End: protocol.Position{
						Line:      entry.start_position.Row,
						Character: entry.start_position.Column,
					},
				},
			},
		})
	}

	for _, entry := range lsp.SelectorEntries[path] {
		items = append(items, protocol.SymbolInformation{
			Name: entry.name,
			Kind: protocol.SymbolKindClass,
			Location: protocol.Location{
				URI: uri.URI("file://" + path),
				Range: protocol.Range{
					Start: protocol.Position{
						Line:      entry.start_position.Row,
						Character: entry.start_position.Column,
					},
					End: protocol.Position{
						Line:      entry.start_position.Row,
						Character: entry.start_position.Column,
					},
				},
			},
		})
	}
	return items
}
func (lsp *Lsp) doesCallExist(call_name string) bool {
	look_in_these := []map[string][]isDefined{lsp.Mixins, lsp.Functions, lsp.Variables}
	for path := range lsp.Trees {
		for _, current_array := range look_in_these {
			for _, entry := range current_array[path] {
				if call_name == entry.name {
					return true
				}
			}
		}
	}
	return false
}

func (lsp *Lsp) reportDiagnostics(path string) {
	diagnostics := []protocol.Diagnostic{}
	for _, entry := range lsp.Calls[path] {
		for _, call := range lsp.CallWhitelist {
			if call == entry.name {
				return
			}
		}
		if !lsp.doesCallExist(entry.name) {
			diagnostic := protocol.Diagnostic{
				Range: protocol.Range{
					Start: protocol.Position{
						Line:      entry.start_position.Row,
						Character: entry.start_position.Column,
					},
					End: protocol.Position{
						Line:      entry.end_position.Row,
						Character: entry.end_position.Column,
					},
				},
				Severity:        protocol.DiagnosticSeverityError,
				Code:            nil,
				CodeDescription: &protocol.CodeDescription{},
				Source:          "SCSS-LSP",
				Message:         "undefined",
			}
			diagnostics = append(diagnostics, diagnostic)
		}
	}
	lsp.SendDiagnostic(path, &diagnostics)
}

func (lsp *Lsp) getReferences(path string, position sitter.Point) []protocol.Location {
	references := []protocol.Location{}
	byte_input, err := lsp.bytesFromFilePath(path)

	if err != nil {
		return references
	}
	root := lsp.Trees[path].RootNode()
	node := root.NamedDescendantForPointRange(position, position)
	word := node.Content(*byte_input)

	for path := range lsp.Trees {
		for _, entry := range lsp.Calls[path] {
			if entry.name == word {
				references = append(references, protocol.Location{
					URI: uri.URI("file://" + path),
					Range: protocol.Range{
						Start: protocol.Position{
							Line:      entry.start_position.Row,
							Character: entry.start_position.Column,
						},
						End: protocol.Position{
							Line:      entry.end_position.Row,
							Character: entry.end_position.Column,
						},
					},
				})
			}
		}
	}
	return references
}

func (lsp *Lsp) gatherSymbols() []protocol.SymbolInformation {
	items := make([]protocol.SymbolInformation, 0)
	for path := range lsp.Trees {
		items = append(items, lsp.gatherSymbolsInPath(path)...)
	}
	return items
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
		}()
		path := replyParams.RootURI.Filename()
		lsp.reportDiagnostics(path)
		return reply(ctx, protocol.InitializeResult{
			Capabilities: protocol.ServerCapabilities{
				// this doesnt work as good as i expected, but it works
				WorkspaceSymbolProvider: true,
				// this works quite good but if multiple lsps are runnning then it will
				// only show info from one of them, at least in nvim
				DocumentSymbolProvider: true,
				DefinitionProvider:     true,
				ReferencesProvider:     true,
				HoverProvider:          true,
				CompletionProvider: &protocol.CompletionOptions{
					ResolveProvider:   false,
					TriggerCharacters: []string{"$", "@"},
				},
				TextDocumentSync: protocol.TextDocumentSyncOptions{
					Change:    protocol.TextDocumentSyncKindFull,
					OpenClose: true,
					WillSave:  true,
					Save: &protocol.SaveOptions{
						IncludeText: true,
					},
				},
			},
		}, nil)

	case protocol.MethodTextDocumentDidOpen:
		params := req.Params()
		var replyParams protocol.DidOpenTextDocumentParams
		err := json.Unmarshal(params, &replyParams)
		if err != nil {
			lsp.Log(err.Error(), protocol.MessageTypeError)
			return nil
		}
		path := replyParams.TextDocument.URI.Filename()
		lsp.reportDiagnostics(path)
		return reply(ctx, nil, nil)

	case protocol.MethodWorkspaceSymbol:
		params := req.Params()
		var replyParams protocol.WorkspaceSymbolParams
		err := json.Unmarshal(params, &replyParams)
		if err != nil {
			lsp.Log(err.Error(), protocol.MessageTypeError)
			return nil
		}
		symbols := lsp.gatherSymbols()
		return reply(ctx, symbols, nil)

	case protocol.MethodTextDocumentDocumentSymbol:
		params := req.Params()
		var replyParams protocol.DocumentSymbolParams
		err := json.Unmarshal(params, &replyParams)
		if err != nil {
			lsp.Log(err.Error(), protocol.MessageTypeError)
			return nil
		}
		path := replyParams.TextDocument.URI.Filename()
		symbols := lsp.gatherSymbolsInPath(path)
		return reply(ctx, symbols, nil)

	case protocol.MethodTextDocumentReferences:
		params := req.Params()
		var replyParams protocol.ReferenceParams
		err := json.Unmarshal(params, &replyParams)
		if err != nil {
			lsp.Log(err.Error(), protocol.MessageTypeError)
		}
		path := replyParams.TextDocument.URI.Filename()

		tree_point := sitter.Point{
			Row:    replyParams.Position.Line,
			Column: replyParams.Position.Character,
		}

		references := lsp.getReferences(path, tree_point)
		if len(references) == 0 {
			return reply(ctx, fmt.Errorf("no references"), nil)
		}
		return reply(ctx, references, nil)

	case protocol.MethodTextDocumentDidSave:
		params := req.Params()
		var replyParams protocol.DidSaveTextDocumentParams
		err := json.Unmarshal(params, &replyParams)
		if err != nil {
			lsp.Log(err.Error(), protocol.MessageTypeError)
			return nil
		}

		path := replyParams.TextDocument.URI.Filename()
		input := []byte(replyParams.Text)
		// TODO figure out how to calculate the diff effectively
		// lets just update the tree and ignore the old one for now
		// calculating the diff is probably more expensive that parsing it again
		tree, err := lsp.Parser.ParseBytes(&input, nil)
		if err != nil {
			lsp.Log("parse_error", protocol.MessageTypeError)
			return reply(ctx, fmt.Errorf("goodbye"), fmt.Errorf("error parsing tree of: %s", path))
		}

		if lsp.Trees[path] == nil {
			lsp.Trees[path] = &sitter.Tree{}
		}
		lsp.Trees[path] = tree
		lsp.UpdateTree(lsp.Trees[path], path, &input)
		lsp.reportDiagnostics(path)
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
		if hover_info == "" {
			return reply(ctx, fmt.Errorf("no res"), nil)
		}
		return reply(ctx, protocol.Hover{
			Contents: protocol.MarkupContent{
				Kind:  protocol.Markdown,
				Value: hover_info,
			},
			Range: &protocol.Range{},
		}, nil)

	case protocol.MethodTextDocumentDefinition:
		params := req.Params()
		var replyParams protocol.DefinitionParams
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

		definition_info := lsp.GetDefinitionInfo(path, tree_point)
		if definition_info == nil {
			// lsp.Log(fmt.Sprintf("no definitions"), protocol.MessageTypeError)
			return reply(ctx, fmt.Errorf("no res"), nil)
		}
		return reply(ctx, definition_info, nil)

	case protocol.MethodTextDocumentDidChange:
		params := req.Params()
		var replyParams protocol.DidChangeTextDocumentParams
		err := json.Unmarshal(params, &replyParams)
		if err != nil {
			return reply(ctx, fmt.Errorf("?"), nil)
		}
		path := replyParams.TextDocument.URI.Filename()
		text := []byte(replyParams.ContentChanges[0].Text)

		lsp.Cache[path] = text

		if lsp.Trees[path] != nil {
			_, err := lsp.UpdateTreeBytes(path, &text)
			if err != nil {
				lsp.Log(err.Error(), protocol.MessageTypeError)
				return reply(ctx, fmt.Errorf("goodbye"), nil)
			}
		}
		return reply(ctx, fmt.Errorf("goodbye"), nil)

	case protocol.MethodTextDocumentCompletion:
		params := req.Params()
		var replyParams protocol.CompletionParams
		err := json.Unmarshal(params, &replyParams)
		if err != nil {
			return reply(ctx, fmt.Errorf("?"), nil)
		}
		position := replyParams.Position
		is_incomplete := replyParams.Context.TriggerKind == protocol.CompletionTriggerKindTriggerForIncompleteCompletions

		prefix := ""
		column := position.Character
		tree := lsp.Trees[replyParams.TextDocument.URI.Filename()]
		if tree == nil {
			lsp.ParseAndSaveTree(replyParams.TextDocument.URI.Filename())
			tree = lsp.Trees[replyParams.TextDocument.URI.Filename()]
		}
		input, err := lsp.bytesFromFilePath(replyParams.TextDocument.URI.Filename())
		if err != nil {
			return reply(ctx, fmt.Errorf("error reading file"), nil)
		}
		input_string := string(*input)
		prefix, err = lsp.getWordAtPosition(&input_string, int(position.Line), int(column)-1)

		if err != nil {
			lsp.Log(err.Error(), protocol.MessageTypeError)
		}
		trigger_character := replyParams.Context.TriggerCharacter
		items := []protocol.CompletionItem{}

		if strings.Contains(prefix, "$") {
			is_incomplete = true
		}

		if trigger_character == "@" {
			for path := range lsp.Mixins {
				for _, entry := range lsp.Mixins[path] {
					items = append(items, protocol.CompletionItem{
						Label:         entry.name,
						Kind:          protocol.CompletionItemKindInterface,
						Documentation: entry.body + "\n\n" + path,
						InsertText:    "include " + entry.name,
					})
				}
			}

			for path := range lsp.Functions {
				for _, entry := range lsp.Functions[path] {
					items = append(items, protocol.CompletionItem{
						Label:         entry.name,
						Kind:          protocol.CompletionItemKindFunction,
						Documentation: entry.body + "\n\n" + path,
						InsertText:    entry.name,
					})
				}
			}
		}

		if len(prefix) > 2 {
			is_incomplete = true
			prefix = prefix[1:]
			for path := range lsp.Variables {
				for _, entry := range lsp.Variables[path] {
					if !strings.Contains(entry.name, prefix) {
						continue
					}
					items = append(items, protocol.CompletionItem{
						Label:         entry.name,
						Kind:          protocol.CompletionItemKindVariable,
						Documentation: entry.body + "\n\n" + path,
						InsertText:    entry.name,
					})
				}
			}
		}
		return reply(ctx, protocol.CompletionList{
			IsIncomplete: is_incomplete,
			Items:        items,
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

func (lsp *Lsp) SendDiagnostic(path string, diagnostics *[]protocol.Diagnostic) {
	lsp.RootConn.Notify(context.Background(), protocol.MethodTextDocumentPublishDiagnostics, protocol.PublishDiagnosticsParams{
		URI:         uri.URI("file://" + path),
		Version:     0,
		Diagnostics: *diagnostics,
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
