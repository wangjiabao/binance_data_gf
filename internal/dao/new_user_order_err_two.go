// =================================================================================
// This is auto-generated by GoFrame CLI tool only once. Fill this file as you wish.
// =================================================================================

package dao

import (
	"binance_data_gf/internal/dao/internal"
)

// internalNewUserOrderErrTwoDao is internal type for wrapping internal DAO implements.
type internalNewUserOrderErrTwoDao = *internal.NewUserOrderErrTwoDao

// newUserOrderErrTwoDao is the data access object for table new_user_order_err_two.
// You can define custom methods on it to extend its functionality as you wish.
type newUserOrderErrTwoDao struct {
	internalNewUserOrderErrTwoDao
}

var (
	// NewUserOrderErrTwo is globally public accessible object for table new_user_order_err_two operations.
	NewUserOrderErrTwo = newUserOrderErrTwoDao{
		internal.NewNewUserOrderErrTwoDao(),
	}
)

// Fill with you ideas below.
