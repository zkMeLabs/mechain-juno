package modules

import (
	"context"
	"encoding/json"
	"strings"

	tmctypes "github.com/cometbft/cometbft/rpc/core/types"
	tmtypes "github.com/cometbft/cometbft/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/authz"
	"github.com/go-co-op/gocron"

	"github.com/forbole/juno/v4/common"
	"github.com/forbole/juno/v4/types"
)

type GetTmcValidators func(height int64) (*tmctypes.ResultValidators, error)

// Module represents a generic module without any particular handling of data
type Module interface {
	// Name returns the module name
	Name() string
}

// Modules represents a slice of Module objects
type Modules []Module

// FindByName returns the module having the given name inside the m slice.
// If no modules are found, returns nil and false.
func (m Modules) FindByName(name string) (module Module, found bool) {
	for _, m := range m {
		if strings.EqualFold(m.Name(), name) {
			return m, true
		}
	}
	return nil, false
}

// --------------------------------------------------------------------------------------------------------------------

type PrepareTablesModule interface {
	// PrepareTables creates tables required by the module.
	PrepareTables() error
	// AutoMigrate will migrate your schema, to keep your schema up to date.
	AutoMigrate() error
}

type AdditionalOperationsModule interface {
	// RunAdditionalOperations runs all the additional operations required by the module.
	// This is the perfect place where to initialize all the operations that subscribe to websockets or other
	// external sources.
	// NOTE. This method will only be run ONCE before starting the parsing of the blocks.
	RunAdditionalOperations() error
}

type AsyncOperationsModule interface {
	// RunAsyncOperations runs all the async operations associated with a module.
	// This method will be run on a separate goroutine, that will stop only when the user stops the entire process.
	// For this reason, this method cannot return an error, and all raised errors should be signaled by panicking.
	RunAsyncOperations()
}

type PeriodicOperationsModule interface {
	// RegisterPeriodicOperations allows to register all the operations that will be run on a periodic basis.
	// The given scheduler can be used to define the periodicity of each task.
	// NOTE. This method will only be run ONCE during the module initialization.
	RegisterPeriodicOperations(scheduler *gocron.Scheduler) error
}

type FastSyncModule interface {
	// DownloadState allows to download the module state at the given height.
	// This will be called only when the fast sync is used, and only once for the initial height.
	// It should query the gRPC and get all the possible data.
	// NOTE. If an error is returned, following modules will still be called.
	DownloadState(height int64) error
}

type GenesisModule interface {
	// HandleGenesis allows to handle the genesis state.
	// For convenience of use, the already-unmarshalled AppState is provided along with the full GenesisDoc.
	// NOTE. The returned error will be logged using the GenesisError method. All other modules' handlers
	// will still be called.
	HandleGenesis(doc *tmtypes.GenesisDoc, appState map[string]json.RawMessage) error
}

type BlockModule interface {
	// HandleBlock allows to handle a single block.
	// For convenience of use, all the transactions present inside the given block will be passed as well.
	// For each transaction present inside the block, HandleTx will be called as well.
	// NOTE. The returned error will be logged using the BlockError method. All other modules' handlers
	// will still be called.
	HandleBlock(block *tmctypes.ResultBlock, results *tmctypes.ResultBlockResults, txs []*types.Tx, getTmcValidators GetTmcValidators) error
}

type TransactionModule interface {
	// HandleTx handles a single transaction.
	// For each message present inside the transaction, HandleMsg will be called as well.
	// NOTE. The returned error will be logged using the TxError method. All other modules' handlers
	// will still be called.
	HandleTx(tx *types.Tx) error
}

type MessageModule interface {
	// HandleMsg handles a single message.
	// For convenience of use, the index of the message inside the transaction and the transaction itself
	// are passed as well.
	// NOTE. The returned error will be logged using the MsgError method. All other modules' handlers
	// will still be called.
	HandleMsg(block *tmctypes.ResultBlock, index int, msg sdk.Msg, tx *types.Tx) error
}

type AuthzMessageModule interface {
	// HandleMsgExec handles a single message that is contained within an authz.MsgExec instance.
	// For convenience of use, the index of the message inside the transaction and the transaction itself
	// are passed as well.
	// NOTE. The returned error will be logged using the MsgError method. All other modules' handlers
	// will still be called.
	HandleMsgExec(index int, msgExec *authz.MsgExec, authzMsgIndex int, executedMsg sdk.Msg, tx *types.Tx) error
}

type EventModule interface {
	//HandleEvent index param here to save possible sequence order
	HandleEvent(ctx context.Context, block *tmctypes.ResultBlock, txHash common.Hash, event sdk.Event) error
	ExtractEventStatements(ctx context.Context, block *tmctypes.ResultBlock, txHash common.Hash, event sdk.Event) (map[string][]interface{}, error)
	SetCtx(key string, value interface{})
	GetCtx(key string) interface{}
	ClearCtx()
}

type EpochModule interface {
	IsProcessed(height uint64) (bool, error)
}