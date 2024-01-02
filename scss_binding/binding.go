package scss_binding

/*
#include "parser.h"
TSLanguage *tree_sitter_scss();
*/
import "C"

import (
	sitter "github.com/smacker/go-tree-sitter"
	"unsafe"
)

func GetLanguage() *sitter.Language {
	ptr := unsafe.Pointer(C.tree_sitter_scss())
	return sitter.NewLanguage(ptr)
}
