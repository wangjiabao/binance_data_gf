package logic

import (
	"binance_data_gf/internal/model/entity"
	"binance_data_gf/internal/service"
	"context"
	"github.com/gogf/gf/v2/frame/g"
	"strconv"
)

type (
	sNewBinancePosition struct{}
)

func init() {
	service.RegisterNewBinancePosition(New())
}

func New() *sNewBinancePosition {
	return &sNewBinancePosition{}
}

func (s *sNewBinancePosition) GetByTraderNumNotClosed(ctx context.Context, traderNum uint64) (binanceTradeHistoryNewestGroup []*entity.NewBinanceTradeHistory, err error) {
	err = g.Model("new_binance_" + strconv.FormatUint(traderNum, 10) + "_position").Ctx(ctx).OrderDesc("id").Scan(&binanceTradeHistoryNewestGroup)
	if nil != err {
		return binanceTradeHistoryNewestGroup, err
	}
	return binanceTradeHistoryNewestGroup, err
}
