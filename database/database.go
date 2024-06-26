package database

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"cosmossdk.io/simapp/params"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"

	"github.com/forbole/juno/v4/common"
	databaseconfig "github.com/forbole/juno/v4/database/config"
	"github.com/forbole/juno/v4/log"
	"github.com/forbole/juno/v4/models"
	"github.com/forbole/juno/v4/types"
)

// Database represents an abstract database that can be used to save data inside it
type Database interface {
	// PrepareTables create tables
	PrepareTables(ctx context.Context, tables []schema.Tabler) error

	// AutoMigrate Automatically migrate your schema, to keep your schema up to date.
	AutoMigrate(ctx context.Context, tables []schema.Tabler) error

	// HasBlock tells whether the database has already stored the block having the given height.
	// An error is returned if the operation fails.
	HasBlock(ctx context.Context, height uint64) (bool, error)

	// GetLastBlockHeight returns the last block height stored in database..
	// An error is returned if the operation fails.
	GetLastBlockHeight(ctx context.Context) (uint64, error)

	// GetMissingHeights returns a slice of missing block heights between startHeight and endHeight
	GetMissingHeights(ctx context.Context, startHeight, endHeight uint64) []uint64

	// SaveBlock will be called when a new block is parsed, passing the block itself
	// and the transactions contained inside that block.
	// An error is returned if the operation fails.
	// NOTE. For each transaction inside txs, SaveTx will be called as well.
	SaveBlock(ctx context.Context, block *models.Block) error

	// GetTotalBlocks returns total number of blocks stored in database.
	GetTotalBlocks(ctx context.Context) int64

	// SaveTx will be called to save each transaction contained inside a block.
	// An error is returned if the operation fails.
	SaveTx(ctx context.Context, blockTimestamp uint64, index int, tx *types.Tx) error

	// SaveCommitSignatures stores a  slice of validator commit signatures.
	// An error is returned if the operation fails.
	SaveCommitSignatures(ctx context.Context, signatures []*types.CommitSig) error

	// SaveBucket will be called to save each bucket contained inside a block.
	// An error is returned if the operation fails.
	SaveBucket(ctx context.Context, bucket *models.Bucket) error

	// UpdateBucket will be called to save each bucket contained inside a block.
	// An error is returned if the operation fails.
	UpdateBucket(ctx context.Context, bucket *models.Bucket) error

	// SaveObject will be called to save each object contained inside a block.
	// An error is returned if the operation fails.
	SaveObject(ctx context.Context, object *models.Object) error

	// UpdateObject will be called to update each object contained inside a block.
	// An error is returned if the operation fails.
	UpdateObject(ctx context.Context, object *models.Object) error

	// GetObject returns an object model with given objectId.
	// It should return only one record
	GetObject(ctx context.Context, objectId common.Hash) (*models.Object, error)

	SaveEpoch(ctx context.Context, epoch *models.Epoch) error

	GetEpoch(ctx context.Context) (*models.Epoch, error)

	// SavePaymentAccount will be called to save PaymentAccount.
	// An error is returned if the operation fails.
	SavePaymentAccount(ctx context.Context, paymentAccount *models.PaymentAccount) error

	// SaveStreamRecord will be called to save SaveStreamRecord.
	// An error is returned if the operation fails.
	SaveStreamRecord(ctx context.Context, streamRecord *models.StreamRecord) error

	// SavePermission will be called to save each policy contained inside a event.
	// An error is returned if the operation fails.
	SavePermission(ctx context.Context, permission *models.Permission) error

	// UpdatePermission will be called to update each policy
	// An error is returned if the operation fails.
	UpdatePermission(ctx context.Context, permission *models.Permission) error

	// CreateGroup will be called to save each group contained inside an event.
	// An error is returned if the operation fails.
	CreateGroup(ctx context.Context, groupMembers []*models.Group) error

	// UpdateGroup will be called to update each group
	// An error is returned if the operation fails.
	UpdateGroup(ctx context.Context, group *models.Group) error

	// DeleteGroup will be called to delete each group
	// An error is returned if the operation fails.
	DeleteGroup(ctx context.Context, group *models.Group) error

	// CreateStorageProvider will be called to save each sp contained inside an event.
	// An error is returned if the operation fails.
	CreateStorageProvider(ctx context.Context, storageProvider *models.StorageProvider) error

	// UpdateStorageProvider will be called to update each sp
	// An error is returned if the operation fails.
	UpdateStorageProvider(ctx context.Context, storageProvider *models.StorageProvider) error

	// MultiSaveStatement will be called to save each statement contained inside a policy.
	// An error is returned if the operation fails.
	MultiSaveStatement(ctx context.Context, statements []*models.Statements) error

	RemoveStatements(ctx context.Context, policyID common.Hash) error

	SaveGVG(ctx context.Context, gvg *models.GlobalVirtualGroup) error

	UpdateGVG(ctx context.Context, gvg *models.GlobalVirtualGroup) error

	SaveLVG(ctx context.Context, lvg *models.LocalVirtualGroup) error

	UpdateLVG(ctx context.Context, lvg *models.LocalVirtualGroup) error

	SaveVGF(ctx context.Context, vgf *models.GlobalVirtualGroupFamily) error

	UpdateVGF(ctx context.Context, vgf *models.GlobalVirtualGroupFamily) error

	SaveDBStatistics(ctx context.Context, ds *models.DataStat) error

	// Begin begins a transaction with any transaction options opts
	Begin(ctx context.Context) *Impl

	// Rollback rollbacks the changes in a transaction
	Rollback()

	// Commit commits the changes in a transaction
	// An error is returned if the operation fails.
	Commit() error

	// Close closes the connection to the database
	Close()
}

// PruningDb represents a database that supports pruning properly
type PruningDb interface {
	// Prune prunes the data for the given height, returning any error
	Prune(height int64) error

	// StoreLastPruned saves the last height at which the database was pruned
	StoreLastPruned(height int64) error

	// GetLastPruned returns the last height at which the database was pruned
	GetLastPruned() (int64, error)
}

// Context contains the data that might be used to build a Database instance
type Context struct {
	Cfg            databaseconfig.Config
	EncodingConfig *params.EncodingConfig
}

// NewContext allows to build a new Context instance
func NewContext(cfg databaseconfig.Config, encodingConfig *params.EncodingConfig) *Context {
	return &Context{
		Cfg:            cfg,
		EncodingConfig: encodingConfig,
	}
}

// Builder represents a method that allows to build any database from a given Marshaler and configuration
type Builder func(ctx *Context) (Database, error)

type Impl struct {
	Db             *gorm.DB
	EncodingConfig *params.EncodingConfig
}

// createPartitionIfNotExists creates a new partition having the given partition id if not existing
func (db *Impl) createPartitionIfNotExists(table string, partitionID int64) error {
	partitionTable := fmt.Sprintf("%s_%d", table, partitionID)

	stmt := fmt.Sprintf(
		"CREATE TABLE IF NOT EXISTS %s PARTITION OF %s FOR VALUES IN (%d)",
		partitionTable,
		table,
		partitionID,
	)
	err := db.Db.Exec(stmt).Error

	if err != nil {
		return err
	}

	return nil
}

// -------------------------------------------------------------------------------------------------------------------

func (db *Impl) PrepareTables(ctx context.Context, tables []schema.Tabler) error {
	q := db.Db.WithContext(ctx)
	m := db.Db.Migrator()

	for _, t := range tables {
		if m.HasTable(t.TableName()) {
			continue
		}

		if err := q.Table(t.TableName()).AutoMigrate(t); err != nil {
			log.Errorw("migrate table failed", "table", t.TableName(), "err", err)
			return err
		}
	}

	return nil
}

func (db *Impl) AutoMigrate(ctx context.Context, tables []schema.Tabler) error {
	m := db.Db.Migrator()
	for _, t := range tables {
		if err := m.AutoMigrate(t); err != nil {
			log.Errorw("migrate table failed", "table", t.TableName(), "err", err)
			return err
		}
	}
	return nil
}

// HasBlock implements database.Database
func (db *Impl) HasBlock(ctx context.Context, height uint64) (bool, error) {
	var res bool
	err := db.Db.Raw(`SELECT EXISTS(SELECT 1 FROM blocks WHERE height = ?);`, height).Scan(&res).Error
	return res, err
}

// GetLastBlockHeight returns the last block height stored inside the database
func (db *Impl) GetLastBlockHeight(ctx context.Context) (uint64, error) {
	var height uint64

	err := db.Db.Table((&models.Block{}).TableName()).Select("height").Order("height DESC").Take(&height).Error
	if errIsNotFound(err) {
		return 0, nil
	}

	return height, err
}

// SaveBlock implements database.Database
func (db *Impl) SaveBlock(ctx context.Context, block *models.Block) error {
	err := db.Db.Table((&models.Block{}).TableName()).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "hash"}},
		UpdateAll: true,
	}, clause.OnConflict{
		Columns:   []clause.Column{{Name: "height"}},
		UpdateAll: true,
	}).Create(block).Error
	return err
}

// GetTotalBlocks implements database.Database
func (db *Impl) GetTotalBlocks(ctx context.Context) int64 {
	var blockCount int64
	err := db.Db.Table((&models.Block{}).TableName()).Count(&blockCount).Error
	if err != nil {
		return 0
	}

	return blockCount
}

// SaveTx implements database.Database
func (db *Impl) SaveTx(ctx context.Context, blockTimestamp uint64, index int, tx *types.Tx) error {
	var sigs = make([]string, len(tx.Signatures))
	for index, sig := range tx.Signatures {
		sigs[index] = base64.StdEncoding.EncodeToString(sig)
	}

	var msgs = make([]string, len(tx.Body.Messages))
	for index, msg := range tx.Body.Messages {
		bz, err := db.EncodingConfig.Codec.MarshalJSON(msg)
		if err != nil {
			return err
		}
		msgs[index] = string(bz)
	}
	msgsBz := fmt.Sprintf("[%s]", strings.Join(msgs, ","))

	feeBz, err := db.EncodingConfig.Codec.MarshalJSON(tx.AuthInfo.Fee)
	if err != nil {
		return fmt.Errorf("failed to JSON encode tx fee: %s", err)
	}

	var sigInfos = make([]string, len(tx.AuthInfo.SignerInfos))
	for index, info := range tx.AuthInfo.SignerInfos {
		bz, err := db.EncodingConfig.Codec.MarshalJSON(info)
		if err != nil {
			return err
		}
		sigInfos[index] = string(bz)
	}
	sigInfoBz := fmt.Sprintf("[%s]", strings.Join(sigInfos, ","))

	logsBz, err := db.EncodingConfig.Amino.MarshalJSON(tx.Logs)
	if err != nil {
		return err
	}

	dbTx := &models.Tx{
		Hash:        common.HexToHash(tx.TxHash),
		Height:      uint64(tx.Height),
		TxIndex:     uint32(index),
		Success:     tx.Successful(),
		Messages:    msgsBz,
		Memo:        tx.Body.Memo,
		Signatures:  strings.Join(sigs, ","),
		SignerInfos: sigInfoBz,
		Fee:         string(feeBz),
		GasWanted:   uint64(tx.GasWanted),
		GasUsed:     uint64(tx.GasUsed),
		RawLog:      tx.RawLog,
		Logs:        string(logsBz),
		Timestamp:   blockTimestamp,
	}

	err = db.Db.Table((&models.Tx{}).TableName()).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "hash"}},
		UpdateAll: true,
	}, clause.OnConflict{
		Columns:   []clause.Column{{Name: "height"}, {Name: "tx_index"}},
		UpdateAll: true,
	}).Create(dbTx).Error
	return err
}

// SaveCommitSignatures implements database.Database
func (db *Impl) SaveCommitSignatures(ctx context.Context, signatures []*types.CommitSig) error {
	if len(signatures) == 0 {
		return nil
	}

	stmt := `INSERT INTO pre_commit (validator_address, height, timestamp, voting_power, proposer_priority) VALUES `

	var sparams []interface{}
	for i, sig := range signatures {
		si := i * 5

		stmt += fmt.Sprintf("($%d, $%d, $%d, $%d, $%d),", si+1, si+2, si+3, si+4, si+5)
		sparams = append(sparams, sig.ValidatorAddress, sig.Height, sig.Timestamp, sig.VotingPower, sig.ProposerPriority)
	}

	stmt = stmt[:len(stmt)-1]
	stmt += " ON CONFLICT (validator_address, timestamp) DO NOTHING"
	err := db.Db.WithContext(ctx).Exec(stmt, sparams...).Error
	return err
}

func (db *Impl) SaveBucket(ctx context.Context, bucket *models.Bucket) error {
	err := db.Db.WithContext(ctx).Table((&models.Bucket{}).TableName()).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "bucket_id"}},
		UpdateAll: true,
	}).Create(bucket).Error
	return err
}

func (db *Impl) UpdateBucket(ctx context.Context, bucket *models.Bucket) error {
	err := db.Db.WithContext(ctx).Table((&models.Bucket{}).TableName()).Where("bucket_id = ?", bucket.BucketID).Updates(bucket).Error
	return err
}

func (db *Impl) SaveObject(ctx context.Context, object *models.Object) error {
	err := db.Db.WithContext(ctx).Table((&models.Object{}).TableName()).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "object_id"}},
		UpdateAll: true,
	}).Create(object).Error
	return err
}

func (db *Impl) UpdateObject(ctx context.Context, object *models.Object) error {
	err := db.Db.WithContext(ctx).Table((&models.Object{}).TableName()).Where("object_id = ?", object.ObjectID).Updates(object).Error
	return err
}

func (db *Impl) GetObject(ctx context.Context, objectId common.Hash) (*models.Object, error) {
	var object models.Object

	err := db.Db.WithContext(ctx).Where(
		"object_id = ? AND removed IS NOT TRUE", objectId).Find(&object).Error
	if err != nil {
		return nil, err
	}
	return &object, nil
}

func (db *Impl) SaveStreamRecord(ctx context.Context, streamRecord *models.StreamRecord) error {
	err := db.Db.WithContext(ctx).Table((&models.StreamRecord{}).TableName()).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "account"}},
		UpdateAll: true,
	}).Create(streamRecord).Error
	return err
}

func (db *Impl) SavePaymentAccount(ctx context.Context, paymentAccount *models.PaymentAccount) error {
	err := db.Db.WithContext(ctx).Table((&models.PaymentAccount{}).TableName()).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "addr"}},
		UpdateAll: true,
	}).Create(paymentAccount).Error
	return err
}

func (db *Impl) SaveEpoch(ctx context.Context, epoch *models.Epoch) error {
	err := db.Db.Table((&models.Epoch{}).TableName()).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "one_row_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"block_height", "block_hash", "update_time"}),
	}).Create(epoch).Error
	return err
}

func (db *Impl) GetEpoch(ctx context.Context) (*models.Epoch, error) {
	var epoch models.Epoch

	err := db.Db.Find(&epoch).Error
	if err != nil && !errIsNotFound(err) {
		return nil, err
	}
	return &epoch, nil
}

func (db *Impl) SavePermission(ctx context.Context, permission *models.Permission) error {
	return db.Db.WithContext(ctx).Table((&models.Permission{}).TableName()).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "principal_type"}, {Name: "principal_value"}, {Name: "resource_type"}, {Name: "resource_id"}},
		UpdateAll: true,
	}).Create(permission).Error
}

func (db *Impl) UpdatePermission(ctx context.Context, permission *models.Permission) error {
	return db.Db.WithContext(ctx).Table((&models.Permission{}).TableName()).Where("policy_id = ?", permission.PolicyID).Updates(permission).Error
}

func (db *Impl) CreateGroup(ctx context.Context, groupMembers []*models.Group) error {
	err := db.Db.WithContext(ctx).Table((&models.Group{}).TableName()).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "group_id"}, {Name: "account_id"}},
		UpdateAll: true,
	}).Create(groupMembers).Error
	return err
}

func (db *Impl) UpdateGroup(ctx context.Context, group *models.Group) error {
	return db.Db.WithContext(ctx).Table((&models.Group{}).TableName()).Where("group_id = ? AND account_id = ?", group.GroupID, group.AccountID).Updates(group).Error
}

func (db *Impl) DeleteGroup(ctx context.Context, group *models.Group) error {
	return db.Db.WithContext(ctx).Table((&models.Group{}).TableName()).Where("group_id = ?", group.GroupID).Updates(group).Error
}

func (db *Impl) CreateStorageProvider(ctx context.Context, storageProvider *models.StorageProvider) error {
	err := db.Db.WithContext(ctx).Table((&models.StorageProvider{}).TableName()).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "sp_id"}},
		UpdateAll: true,
	}).Create(storageProvider).Error
	return err
}

func (db *Impl) UpdateStorageProvider(ctx context.Context, storageProvider *models.StorageProvider) error {
	return db.Db.WithContext(ctx).Table((&models.StorageProvider{}).TableName()).Where("sp_id = ? ", storageProvider.SpId).Updates(storageProvider).Error
}

func (db *Impl) MultiSaveStatement(ctx context.Context, statements []*models.Statements) error {
	return db.Db.WithContext(ctx).Table((&models.Statements{}).TableName()).Create(statements).Error
}

func (db *Impl) RemoveStatements(ctx context.Context, policyID common.Hash) error {
	return db.Db.WithContext(ctx).Table((&models.Statements{}).TableName()).Where("policy_id = ?", policyID).Update("removed", true).Error
}

func (db *Impl) SaveGVG(ctx context.Context, gvg *models.GlobalVirtualGroup) error {
	err := db.Db.WithContext(ctx).Table((&models.GlobalVirtualGroup{}).TableName()).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "global_virtual_group_id"}},
		UpdateAll: true,
	}).Create(gvg).Error
	return err
}

func (db *Impl) UpdateGVG(ctx context.Context, gvg *models.GlobalVirtualGroup) error {
	err := db.Db.WithContext(ctx).Table((&models.GlobalVirtualGroup{}).TableName()).Where("global_virtual_group_id = ?", gvg.GlobalVirtualGroupId).Updates(gvg).Error
	return err
}

func (db *Impl) SaveLVG(ctx context.Context, lvg *models.LocalVirtualGroup) error {
	err := db.Db.WithContext(ctx).Table((&models.LocalVirtualGroup{}).TableName()).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "local_virtual_group_id"}},
		UpdateAll: true,
	}).Create(lvg).Error
	return err
}

func (db *Impl) UpdateLVG(ctx context.Context, lvg *models.LocalVirtualGroup) error {
	err := db.Db.WithContext(ctx).Table((&models.LocalVirtualGroup{}).TableName()).Where("local_virtual_group_id = ? and bucket_id = ?", lvg.LocalVirtualGroupId, lvg.BucketID).Updates(lvg).Error
	return err
}

func (db *Impl) SaveVGF(ctx context.Context, vgf *models.GlobalVirtualGroupFamily) error {
	err := db.Db.WithContext(ctx).Table((&models.GlobalVirtualGroupFamily{}).TableName()).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "global_virtual_group_family_id"}},
		UpdateAll: true,
	}).Create(vgf).Error
	return err
}

func (db *Impl) UpdateVGF(ctx context.Context, vgf *models.GlobalVirtualGroupFamily) error {
	err := db.Db.WithContext(ctx).Table((&models.GlobalVirtualGroupFamily{}).TableName()).Where("global_virtual_group_family_id = ?", vgf.GlobalVirtualGroupFamilyId).Updates(vgf).Error
	return err
}

func (db *Impl) SaveDBStatistics(ctx context.Context, ds *models.DataStat) error {
	return nil
}

func (db *Impl) Begin(ctx context.Context) *Impl {
	return &Impl{
		Db: db.Db.WithContext(ctx).Begin(),
	}
}

func (db *Impl) Rollback() {
	db.Db.Rollback()
}

func (db *Impl) Commit() error {
	return db.Db.Commit().Error
}

// Close implements database.Database
func (db *Impl) Close() {
	var err error
	if err != nil {
		log.Errorw("error while closing connection", "err", err)
	}
}

// -------------------------------------------------------------------------------------------------------------------

// GetLastPruned implements database.PruningDb
func (db *Impl) GetLastPruned() (int64, error) {
	var lastPrunedHeight int64
	err := db.Db.Raw(`SELECT coalesce(MAX(last_pruned_height),0) FROM pruning LIMIT 1;`).Scan(&lastPrunedHeight).Error
	return lastPrunedHeight, err
}

// StoreLastPruned implements database.PruningDb
func (db *Impl) StoreLastPruned(height int64) error {
	err := db.Db.Exec(`DELETE FROM pruning`).Error
	if err != nil {
		return err
	}

	err = db.Db.Exec(`INSERT INTO pruning (last_pruned_height) VALUES ($1)`, height).Error
	return err
}

// Prune implements database.PruningDb
func (db *Impl) Prune(height int64) error {
	err := db.Db.Exec(`DELETE FROM pre_commit WHERE height = $1`, height).Error
	if err != nil {
		return err
	}

	err = db.Db.Exec(`
DELETE FROM message 
USING transaction 
WHERE message.transaction_hash = transaction.hash AND transaction.height = $1
`, height).Error
	return err
}

func errIsNotFound(err error) bool {
	return errors.Is(err, sql.ErrNoRows) || errors.Is(err, gorm.ErrRecordNotFound)
}
