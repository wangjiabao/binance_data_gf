package cmd

import (
	"binance_data_gf/internal/model/entity"
	"binance_data_gf/internal/service"
	"context"
	"fmt"
	"github.com/gogf/gf/v2/os/gcmd"
	"github.com/gogf/gf/v2/os/gtimer"
	"time"
)

var (
	Main = &gcmd.Command{
		Name: "main",
	}

	// Trader 监听系统中被拉取数据交易员的人员变更
	Trader = &gcmd.Command{
		Name:  "trader",
		Brief: "listen trader",
		Func: func(ctx context.Context, parser *gcmd.Parser) (err error) {
			// ip池子维护
			initIpUpdateTask(ctx)
			addIpUpdateTask(ctx)

			updateTradersPeriodically(ctx)
			return nil
		},
	}
)

// 全局变量来跟踪定时任务
var (
	traderSingleton = make(map[uint64]*gtimer.Entry)
)

func updateTradersPeriodically(ctx context.Context) {
	// 每分钟查询数据库以更新交易员任务
	interval := time.Minute

	for {
		updateTraders(ctx)
		time.Sleep(interval)
	}
}

func updateTraders(ctx context.Context) {
	newTraderIDs, err := fetchTraderIDsFromDB(ctx)
	if err != nil {
		fmt.Println("查询数据库时出错:", err)
		return
	}

	// 空的情况，这里不会做任何修改，那么手动把程序停掉就行了
	if 0 >= len(newTraderIDs) {
		return
	}

	// 不存在新增
	idMap := make(map[uint64]bool, 0)
	for _, vNewTraderIDs := range newTraderIDs {
		idMap[vNewTraderIDs] = true
		if _, ok := traderSingleton[vNewTraderIDs]; !ok { // 不存在新增
			addTraderTask(ctx, vNewTraderIDs)
		}
	}

	// 反向检测，不存在删除
	for k, _ := range traderSingleton {
		if _, ok := idMap[k]; !ok {
			removeTraderTask(k)
		}
	}
}

func fetchTraderIDsFromDB(ctx context.Context) ([]uint64, error) {
	var (
		err error
	)
	traderNums := make([]uint64, 0)

	traders := make([]*entity.NewBinanceTrader, 0)
	traders, err = service.NewBinanceTrader().GetAllTraders(ctx)
	if nil != err {
		return traderNums, err
	}

	for _, vTraders := range traders {
		traderNums = append(traderNums, vTraders.TraderNum)
	}

	return traderNums, err
}

func initIpUpdateTask(ctx context.Context) {
	err := service.BinanceTraderHistory().UpdateProxyIp(ctx)
	if err != nil {
		fmt.Println("ip更新任务运行时出错:", err)
	}
}

func addIpUpdateTask(ctx context.Context) {
	// 任务
	handle := func(ctx context.Context) {
		err := service.BinanceTraderHistory().UpdateProxyIp(ctx)
		if err != nil {
			fmt.Println("ip更新任务运行时出错:", err)
		}
	}

	// 小于ip最大活性时长
	gtimer.AddSingleton(ctx, time.Minute*20, handle)
}

func addTraderTask(ctx context.Context, traderID uint64) {
	// 任务
	handle := func(ctx context.Context) {
		relTraderId := traderID // go1.22以前有循环变量陷阱，不思考这里是否也会如此，直接用临时变量解决
		err := service.BinanceTraderHistory().PullAndOrder(ctx, relTraderId)
		if err != nil {
			fmt.Println("任务运行时出错:", "交易员信息:", relTraderId, "错误信息:", err)
		}
	}
	traderSingleton[traderID] = gtimer.AddSingleton(ctx, time.Second*3, handle)
	fmt.Println("添加成功交易员:", traderID)
}

func removeTraderTask(traderID uint64) {
	if entry, exists := traderSingleton[traderID]; exists {
		entry.Close()
		delete(traderSingleton, traderID)
		fmt.Println("删除成功交易员:", traderID)
	}
}
