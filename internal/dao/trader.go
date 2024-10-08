// =================================================================================
// This is auto-generated by GoFrame CLI tool only once. Fill this file as you wish.
// =================================================================================

package dao

import (
	"binance_data_gf/internal/dao/internal"
)

// internalTraderDao is internal type for wrapping internal DAO implements.
type internalTraderDao = *internal.TraderDao

// traderDao is the data access object for table trader.
// You can define custom methods on it to extend its functionality as you wish.
type traderDao struct {
	internalTraderDao
}

var (
	// Trader is globally public accessible object for table trader operations.
	Trader = traderDao{
		internal.NewTraderDao(),
	}
)

// Fill with you ideas below.
