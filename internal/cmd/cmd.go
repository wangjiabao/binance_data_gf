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
			serviceBinanceTrader := service.BinanceTraderHistory()
			// 任务1 同步订单
			go func() {
				// ip池子维护
				initIpUpdateTask(ctx, serviceBinanceTrader)
				//addIpUpdateTask(ctx)
				updateTradersPeriodically(ctx, serviceBinanceTrader)
			}()

			// 任务2 监听广播新订单
			go func() {
				initListenAndOrderTask(ctx, serviceBinanceTrader)
			}()

			// 任务2 处理平仓
			go func() {
				for {
					time.Sleep(10 * time.Second) // 依赖ip等待初始化完成，后续完成任务后，间隔10s执行，经验预估10s肯定够用了
					pullAndCloseTask(ctx, serviceBinanceTrader)
				}
			}()

			// 阻塞
			select {}
		},
	}
)

// 全局变量来跟踪定时任务
var (
	traderSingleton = make(map[uint64]*gtimer.Entry)
)

func updateTradersPeriodically(ctx context.Context, serviceBinanceTrader service.IBinanceTraderHistory) {
	// 每分钟查询数据库以更新交易员任务
	interval := time.Minute

	for {
		updateTraders(ctx, serviceBinanceTrader)
		time.Sleep(interval)
	}
}

func updateTraders(ctx context.Context, serviceBinanceTrader service.IBinanceTraderHistory) {
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
			addTraderTask(ctx, vNewTraderIDs, serviceBinanceTrader)
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

func initIpUpdateTask(ctx context.Context, serviceBinanceTrader service.IBinanceTraderHistory) {
	err := serviceBinanceTrader.UpdateProxyIp(ctx)
	if err != nil {
		fmt.Println("ip更新任务运行时出错:", err)
	}
}

func initListenAndOrderTask(ctx context.Context, serviceBinanceTrader service.IBinanceTraderHistory) {
	serviceBinanceTrader.ListenThenOrder(ctx)
}

func pullAndCloseTask(ctx context.Context, serviceBinanceTrader service.IBinanceTraderHistory) {
	serviceBinanceTrader.PullAndClose(ctx)
}

func addIpUpdateTask(ctx context.Context, serviceBinanceTrader service.IBinanceTraderHistory) {
	// 任务
	handle := func(ctx context.Context) {
		err := serviceBinanceTrader.UpdateProxyIp(ctx)
		if err != nil {
			fmt.Println("ip更新任务运行时出错:", err)
		}
	}

	// 小于ip最大活性时长
	gtimer.AddSingleton(ctx, time.Minute*20, handle)
}

func addTraderTask(ctx context.Context, traderID uint64, serviceBinanceTrader service.IBinanceTraderHistory) {
	// 任务
	handle := func(ctx context.Context) {
		relTraderId := traderID // go1.22以前有循环变量陷阱，不思考这里是否也会如此，直接用临时变量解决
		err := serviceBinanceTrader.PullAndOrder(ctx, relTraderId)
		if err != nil {
			fmt.Println("任务运行时出错:", "交易员信息:", relTraderId, "错误信息:", err)
		}
	}
	traderSingleton[traderID] = gtimer.AddSingleton(ctx, time.Second*2, handle)
	fmt.Println("添加成功交易员:", traderID)
}

func removeTraderTask(traderID uint64) {
	if entry, exists := traderSingleton[traderID]; exists {
		entry.Close()
		delete(traderSingleton, traderID)
		fmt.Println("删除成功交易员:", traderID)
	}
}
