// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// NewUserBindTraderTwoDao is the data access object for table new_user_bind_trader_two.
type NewUserBindTraderTwoDao struct {
	table   string                      // table is the underlying table name of the DAO.
	group   string                      // group is the database configuration group name of current DAO.
	columns NewUserBindTraderTwoColumns // columns contains all the column names of Table for convenient usage.
}

// NewUserBindTraderTwoColumns defines and stores column names for table new_user_bind_trader_two.
type NewUserBindTraderTwoColumns struct {
	Id        string // 主键id
	UserId    string // 用户id
	TraderId  string // 交易员id
	Amount    string //
	Status    string // 可用0，不可用1，待更换2
	InitOrder string // 绑后是否初始化仓位
	CreatedAt string // 创建时间
	UpdatedAt string // 更新时间
	Num       string //
}

// newUserBindTraderTwoColumns holds the columns for table new_user_bind_trader_two.
var newUserBindTraderTwoColumns = NewUserBindTraderTwoColumns{
	Id:        "id",
	UserId:    "user_id",
	TraderId:  "trader_id",
	Amount:    "amount",
	Status:    "status",
	InitOrder: "init_order",
	CreatedAt: "created_at",
	UpdatedAt: "updated_at",
	Num:       "num",
}

// NewNewUserBindTraderTwoDao creates and returns a new DAO object for table data access.
func NewNewUserBindTraderTwoDao() *NewUserBindTraderTwoDao {
	return &NewUserBindTraderTwoDao{
		group:   "default",
		table:   "new_user_bind_trader_two",
		columns: newUserBindTraderTwoColumns,
	}
}

// DB retrieves and returns the underlying raw database management object of current DAO.
func (dao *NewUserBindTraderTwoDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of current dao.
func (dao *NewUserBindTraderTwoDao) Table() string {
	return dao.table
}

// Columns returns all column names of current dao.
func (dao *NewUserBindTraderTwoDao) Columns() NewUserBindTraderTwoColumns {
	return dao.columns
}

// Group returns the configuration group name of database of current dao.
func (dao *NewUserBindTraderTwoDao) Group() string {
	return dao.group
}

// Ctx creates and returns the Model for current DAO, It automatically sets the context for current operation.
func (dao *NewUserBindTraderTwoDao) Ctx(ctx context.Context) *gdb.Model {
	return dao.DB().Model(dao.table).Safe().Ctx(ctx)
}

// Transaction wraps the transaction logic using function f.
// It rollbacks the transaction and returns the error from function f if it returns non-nil error.
// It commits the transaction and returns nil if function f returns nil.
//
// Note that, you should not Commit or Rollback the transaction in function f
// as it is automatically handled by this function.
func (dao *NewUserBindTraderTwoDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
