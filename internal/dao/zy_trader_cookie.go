// =================================================================================
// This is auto-generated by GoFrame CLI tool only once. Fill this file as you wish.
// =================================================================================

package dao

import (
	"binance_data_gf/internal/dao/internal"
)

// internalZyTraderCookieDao is internal type for wrapping internal DAO implements.
type internalZyTraderCookieDao = *internal.ZyTraderCookieDao

// zyTraderCookieDao is the data access object for table zy_trader_cookie.
// You can define custom methods on it to extend its functionality as you wish.
type zyTraderCookieDao struct {
	internalZyTraderCookieDao
}

var (
	// ZyTraderCookie is globally public accessible object for table zy_trader_cookie operations.
	ZyTraderCookie = zyTraderCookieDao{
		internal.NewZyTraderCookieDao(),
	}
)

// Fill with you ideas below.
