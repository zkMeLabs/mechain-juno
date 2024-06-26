package parser

import (
	"cosmossdk.io/simapp/params"

	"github.com/forbole/juno/v4/database"
	"github.com/forbole/juno/v4/modules"
	"github.com/forbole/juno/v4/node"
)

// Context represents the context that is shared among different workers
type Context struct {
	EncodingConfig *params.EncodingConfig
	Node           node.Node
	Database       database.Database
	Indexer        Indexer
	Modules        []modules.Module
}

// NewContext builds a new Context instance
func NewContext(
	encodingConfig *params.EncodingConfig,
	proxy node.Node,
	db database.Database,
	modules []modules.Module,
	indexer Indexer,
) *Context {
	return &Context{
		EncodingConfig: encodingConfig,
		Node:           proxy,
		Database:       db,
		Indexer:        indexer,
		Modules:        modules,
	}
}
