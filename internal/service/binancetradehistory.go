// ================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// You can delete these comments if you wish manually maintain this interface file.
// ================================================================================

package service

import (
	"context"
)

type (
	IBinanceTraderHistory interface {
		// UpdateProxyIp ip更新
		UpdateProxyIp(ctx context.Context) (err error)
		// PullAndOrder 拉取binance数据
		PullAndOrder(ctx context.Context, traderNum uint64) (err error)
		// PullAndClose 拉取binance数据
		PullAndClose(ctx context.Context)
		// ListenThenOrder 监听拉取的binance数据
		ListenThenOrder(ctx context.Context)
	}
)

var (
	localBinanceTraderHistory IBinanceTraderHistory
)

func BinanceTraderHistory() IBinanceTraderHistory {
	if localBinanceTraderHistory == nil {
		panic("implement not found for interface IBinanceTraderHistory, forgot register?")
	}
	return localBinanceTraderHistory
}

func RegisterBinanceTraderHistory(i IBinanceTraderHistory) {
	localBinanceTraderHistory = i
}
