package logic

import (
	"binance_data_gf/internal/model/do"
	"binance_data_gf/internal/model/entity"
	"binance_data_gf/internal/service"
	"context"
	"encoding/json"
	"fmt"
	"github.com/gogf/gf/v2/container/gmap"
	"github.com/gogf/gf/v2/container/gqueue"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/grpool"
	"github.com/gogf/gf/v2/os/gtime"
	"io"
	"io/ioutil"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type (
	sBinanceTraderHistory struct {
		// 全局存储
		pool *grpool.Pool
		// 针对binance拉取交易员下单接口定制的数据结构，每页数据即使同一个ip不同的交易员是可以返回数据，在逻辑上和这个设计方式要达成共识再设计后续的程序逻辑。
		// 考虑实际的业务场景，接口最多6000条，每次查50条，最多需要120个槽位。假设，每10个槽位一循环，意味着对应同一个交易员每次并行使用10个ip查询10页数据。
		//ips map[int]*proxyData
		ips        *gmap.IntStrMap
		orderQueue *gqueue.Queue
	}
)

func init() {
	service.RegisterBinanceTraderHistory(New())
}

func New() *sBinanceTraderHistory {
	return &sBinanceTraderHistory{
		grpool.New(), // 这里是请求协程池子，可以配合着可并行请求binance的限制使用，来限制最大共存数，后续jobs都将排队，考虑到上层的定时任务
		gmap.NewIntStrMap(true),
		gqueue.New(), // 下单顺序队列
	}
}

func IsEqual(f1, f2 float64) bool {
	if f1 > f2 {
		return f1-f2 < 0.000000001
	} else {
		return f2-f1 < 0.000000001
	}
}

func lessThanOrEqualZero(a, b float64, epsilon float64) bool {
	return a-b < epsilon || math.Abs(a-b) < epsilon
}

type proxyData struct {
	Ip   string
	Port int64
}

type proxyRep struct {
	Data []*proxyData
}

// 拉取代理列表，暂时弃用
func requestProxy() ([]*proxyData, error) {
	var (
		resp   *http.Response
		b      []byte
		res    []*proxyData
		err    error
		apiUrl = ""
	)

	httpClient := &http.Client{
		Timeout: time.Second * 10,
	}

	// 构造请求
	resp, err = httpClient.Get(apiUrl)
	if err != nil {
		return nil, err
	}

	// 结果
	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			fmt.Println(err)
		}
	}(resp.Body)

	b, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	var l *proxyRep
	err = json.Unmarshal(b, &l)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	res = make([]*proxyData, 0)
	if nil == l.Data || 0 >= len(l.Data) {
		return res, nil
	}

	for _, v := range l.Data {
		res = append(res, v)
	}

	return res, nil
}

func (s *sBinanceTraderHistory) UpdateProxyIp(ctx context.Context) (err error) {
	// 20个客户端代理，这里注意一定是key是从0开始到size-1的
	s.ips.Set(0, "http://43.133.175.121:888/")
	s.ips.Set(1, "http://43.133.209.89:888/")
	s.ips.Set(2, "http://43.133.177.73:888/")
	s.ips.Set(3, "http://43.163.224.155:888/")
	s.ips.Set(4, "http://43.163.210.110:888/")
	s.ips.Set(5, "http://43.163.242.180:888/")
	s.ips.Set(6, "http://43.153.162.213:888/")
	s.ips.Set(7, "http://43.163.197.191:888/")
	s.ips.Set(8, "http://43.163.217.70:888/")
	s.ips.Set(9, "http://43.163.194.135:888/")
	s.ips.Set(10, "http://43.130.254.24:888/")
	s.ips.Set(11, "http://43.128.249.92:888/")
	s.ips.Set(12, "http://43.153.177.107:888/")
	s.ips.Set(13, "http://43.133.15.135:888/")
	s.ips.Set(14, "http://43.130.229.66:888/")
	s.ips.Set(15, "http://43.133.196.220:888/")
	s.ips.Set(16, "http://43.163.208.36:888/")
	s.ips.Set(17, "http://43.133.204.254:888/")
	s.ips.Set(18, "http://43.153.181.89:888/")
	s.ips.Set(19, "http://43.163.231.217:888/")

	return nil

	//res := make([]*proxyData, 0)
	//for i := 0; i < 1000; i++ {
	//	var (
	//		resTmp []*proxyData
	//	)
	//	resTmp, err = requestProxy()
	//	if nil != err {
	//		fmt.Println("ip池子更新出错", err)
	//		time.Sleep(time.Second * 1)
	//		continue
	//	}
	//
	//	if 0 < len(resTmp) {
	//		res = append(res, resTmp...)
	//	}
	//
	//	if 20 <= len(res) { // 20个最小
	//		break
	//	}
	//
	//	fmt.Println("ip池子更新时无数据")
	//	time.Sleep(time.Second * 1)
	//}
	//
	//s.ips.Clear()
	//
	//// 更新
	//for k, v := range res {
	//	s.ips.Set(k, "http://"+v.Ip+":"+strconv.FormatInt(v.Port, 10)+"/")
	//}
	//
	//fmt.Println("ip池子更新成功", time.Now(), s.ips.Size())
	//return nil
}

func (s *sBinanceTraderHistory) PullAndOrder(ctx context.Context, traderNum uint64) (err error) {
	start := time.Now()

	//// 测试部分注释
	//_, err = s.pullAndSetHandle(ctx, traderNum, 120) // 执行
	//fmt.Println("ok", traderNum)
	//return nil

	// 测试部分注释
	//if 1 == traderNum {
	//	fmt.Println("此时系统，workers：", pool.Size(), "jobs：", pool.Jobs())
	//	return nil
	//}

	/**
	 * 这里说明一下同步规则
	 * 首先任务已经是每个交易员的单独协程了
	 *
	 * 步骤1：
	 * 探测任务：拉取第1页10条，前10条与数据库中前10条对比，一摸一样认为无新订单（继续进行探测），否则有新订单进入步骤2。
	 * 步骤2：
	 * 下单任务：并行5个任务，每个任务表示每页数据的拉取，每次5页共250条，拉取后重新拉取第1页数据与刚才的5页中的第1页，
	 * 对比10条，一模一样表示这段任务执行期间无更新，否则全部放弃，重新开始步骤2。
	 *
	 * ip池子的加入
	 * 步骤1中探测任务可以考虑分配一个ip。
	 * 步骤2中每个任务分配不同的ip（防止ip封禁用，目前经验是binance对每个ip在查询每个交易员数据时有2秒的限制，并行则需要不同的ip）
	 *
	 * 数据库不存在数据时，直接执行步骤2，并行5页，如果执行完任务，发现有新订单，则全部放弃，重新步骤2。
	 */

	// 数据库对比数据
	var (
		compareMax                     = 10 // 预设最大对比条数，小于最大限制10条，注意：不能超过50条，在程序多出有写死，binance目前每页最大条数
		currentCompareMax              int  // 实际获得对比条数
		binanceTradeHistoryNewestGroup []*entity.NewBinanceTradeHistory
		resData                        []*entity.NewBinanceTradeHistory
		resDataCompare                 []*entity.NewBinanceTradeHistory
		initPull                       bool
	)

	err = g.Model("new_binance_trade_" + strconv.FormatUint(traderNum, 10) + "_history").Ctx(ctx).Limit(compareMax).OrderDesc("id").Scan(&binanceTradeHistoryNewestGroup)
	if nil != err {
		return err
	}

	currentCompareMax = len(binanceTradeHistoryNewestGroup)

	ipMapNeedWait := make(map[string]bool, 0) // 刚使用的ip，大概率加快查询速度，2s以内别用的ip
	// 数据库无数据，拉取满额6000条数据
	if 0 >= currentCompareMax {
		initPull = true
		resData, err = s.pullAndSetHandle(ctx, traderNum, 120, true, ipMapNeedWait) // 执行
		if nil != err {
			fmt.Println("初始化，执行拉取数据协程异常，错误信息：", err, "交易员：", traderNum)
		}

		if nil == resData {
			fmt.Println("初始化，执行拉取数据协程异常，数据缺失", "交易员：", traderNum)
			return nil
		}

		if 0 >= len(resData) {
			fmt.Println("初始化，执行拉取数据协程异常，空数据：", len(resData), "交易员：", traderNum)
			return nil
		}

		var (
			compareResDiff bool
		)
		// 截取前10条记录
		afterCompare := make([]*entity.NewBinanceTradeHistory, 0)
		if len(resData) >= compareMax {
			afterCompare = resData[:compareMax]
		} else {
			// 这里也限制了最小入库的交易员数据条数
			fmt.Println("初始化，执行拉取数据协程异常，条数不足，条数：", len(resData), "交易员：", traderNum)
			return nil
		}

		if 0 >= len(afterCompare) {
			fmt.Println("初始化，执行拉取数据协程异常，条数不足，条数：", len(resData), "交易员：", traderNum)
			return nil
		}

		// todo 如果未来初始化仓位不准，那这里应该很多的嫌疑，因为在拉取中第一页数据的协程最后执行完，那么即使带单员更新了，也不会被察觉，当然概率很小
		// 有问题的话，可以换成重新执行一次完整的拉取，比较
		_, _, compareResDiff, err = s.compareBinanceTradeHistoryPageOne(int64(compareMax), traderNum, afterCompare)
		if nil != err {
			return err
		}

		// 不同，则拉取数据的时间有新单，放弃操作，等待下次执行
		if compareResDiff {
			fmt.Println("初始化，执行拉取数据协程异常，有新单", "交易员：", traderNum)
			return nil
		}

	} else if compareMax <= currentCompareMax {
		/**
		 * 试探开始
		 * todo
		 * 假设：binance的数据同一时间的数据乱序出现时，因为和数据库前n条不一致，而认为是新数据，后续保存会有处理，但是这里就会一直生效，现在这个假设还未出现。*
		 * 因为保存对上述假设的限制，延迟出现的同一时刻的数据一直不会被系统保存，而每次都会触发这里的比较，得到不同数据，为了防止乱序的假设最后是这样做，但是可能导致一直拉取10页流量增长，后续观察假设不存在最好，假设存在更新方案。
		 */
		var (
			newData        []*binanceTradeHistoryDataList
			compareResDiff bool
		)

		ipMapNeedWait, newData, compareResDiff, err = s.compareBinanceTradeHistoryPageOne(int64(compareMax), traderNum, binanceTradeHistoryNewestGroup)
		if nil != err {
			return err
		}

		// 相同，返回
		if !compareResDiff {
			return nil
		}

		if nil == newData || 0 >= len(newData) {
			fmt.Println("日常，执行拉取数据协程异常，新数据未空，错误信息：", err, "交易员：", traderNum)
			return nil
		}

		// 不同，开始捕获
		resData, err = s.pullAndSetHandle(ctx, traderNum, 10, true, ipMapNeedWait) // todo 执行，目前猜测最大500条，根据经验拍脑袋
		if nil != err {
			fmt.Println("日常，执行拉取数据协程异常，错误信息：", err, "交易员：", traderNum)
		}

		if nil == resData {
			fmt.Println("日常，执行拉取数据协程异常，数据缺失", "交易员：", traderNum)
			return nil
		}

		if 0 >= len(resData) {
			fmt.Println("日常，执行拉取数据协程异常，数据为空", "交易员：", traderNum)
			return nil
		}

		// 重新拉取，比较探测的结果，和最后的锁定结果
		resDataCompare, err = s.pullAndSetHandle(ctx, traderNum, 1, false, ipMapNeedWait) // todo 执行，目前猜测最大500条，根据经验拍脑袋
		if nil != err {
			fmt.Println("日常，执行拉取数据协程异常，比较数据，错误信息：", err, "交易员：", traderNum)
		}

		if nil == resDataCompare {
			fmt.Println("日常，执行拉取数据协程异常，比较数据，数据缺失", "交易员：", traderNum)
			return nil
		}

		for kNewData, vNewData := range newData {
			// 比较
			if !(vNewData.Time == resDataCompare[kNewData].Time &&
				vNewData.Symbol == resDataCompare[kNewData].Symbol &&
				vNewData.Side == resDataCompare[kNewData].Side &&
				vNewData.PositionSide == resDataCompare[kNewData].PositionSide &&
				IsEqual(vNewData.Qty, resDataCompare[kNewData].Qty) && // 数量
				IsEqual(vNewData.Price, resDataCompare[kNewData].Price) && //价格
				IsEqual(vNewData.RealizedProfit, resDataCompare[kNewData].RealizedProfit) &&
				IsEqual(vNewData.Quantity, resDataCompare[kNewData].Quantity) &&
				IsEqual(vNewData.Fee, resDataCompare[kNewData].Fee)) {
				fmt.Println("日常，执行拉取数据协程异常，比较数据，数据不同，第一条", "交易员：", traderNum, resData[0], resDataCompare[0])
				return nil
			}
		}

	} else {
		fmt.Println("执行拉取数据协程异常，查询数据库数据条数不在范围：", compareMax, currentCompareMax, "交易员：", traderNum, "初始化：", initPull)
	}

	// 时长
	fmt.Printf("程序拉取部分，开始 %v, 时长: %v\n", start, time.Since(start))

	// 非初始化，截断数据
	if !initPull {
		tmpResData := make([]*entity.NewBinanceTradeHistory, 0)
		//tmpCurrentCompareMax := currentCompareMax
		for _, vResData := range resData {
			/**
			 * todo
			 * 停下的条件，暂时是：
			 * 1 数据库的最新一条比较，遇到时间，币种，方向一致的订单，认为是已经纳入数据库的。
			 * 2 如果数据库最新的时间已经晚于遍历时遇到的时间也一定停下，这里会有一个好处，即使binance的数据同一时间的数据总是乱序出现时，我们也不会因为和数据库第一条不一致，而认为是新数据。
			 *
			 * 如果存在误判的原因，
			 * 1 情况是在上次执行完拉取保存后，相同时间的数据，因为binance的问题，延迟出现了。
			 */
			if vResData.Time == binanceTradeHistoryNewestGroup[0].Time &&
				vResData.Side == binanceTradeHistoryNewestGroup[0].Side &&
				vResData.PositionSide == binanceTradeHistoryNewestGroup[0].PositionSide &&
				vResData.Symbol == binanceTradeHistoryNewestGroup[0].Symbol {
				break
			}

			if vResData.Time <= binanceTradeHistoryNewestGroup[0].Time {
				fmt.Println("遍历时竟然未发现相同数据！此时数据时间已经小于了数据库最新一条的时间，如果时间相同可能是binance延迟出现的数据，数据：", vResData, binanceTradeHistoryNewestGroup[0])
				break
			}

			//if (len(resData) - k) <= tmpCurrentCompareMax { // 还剩下几条
			//	tmpCurrentCompareMax = len(resData) - k
			//}

			//tmp := 0
			//if 0 < tmpCurrentCompareMax {
			//	for i := 0; i < tmpCurrentCompareMax; i++ { // todo 如果只剩下最大条数以内的数字，只能兼容着比较，这里根据经验判断会不会出现吧
			//		if resData[k+i].Time == binanceTradeHistoryNewestGroup[i].Time &&
			//			resData[k+i].Symbol == binanceTradeHistoryNewestGroup[i].Symbol &&
			//			resData[k+i].Side == binanceTradeHistoryNewestGroup[i].Side &&
			//			resData[k+i].PositionSide == binanceTradeHistoryNewestGroup[i].PositionSide &&
			//			IsEqual(resData[k+i].Qty, binanceTradeHistoryNewestGroup[i].Qty) && // 数量
			//			IsEqual(resData[k+i].Price, binanceTradeHistoryNewestGroup[i].Price) && //价格
			//			IsEqual(resData[k+i].RealizedProfit, binanceTradeHistoryNewestGroup[i].RealizedProfit) &&
			//			IsEqual(resData[k+i].Quantity, binanceTradeHistoryNewestGroup[i].Quantity) &&
			//			IsEqual(resData[k+i].Fee, binanceTradeHistoryNewestGroup[i].Fee) {
			//			tmp++
			//		}
			//	}
			//
			//	if tmpCurrentCompareMax == tmp {
			//		break
			//	}
			//} else {
			//	break
			//}

			tmpResData = append(tmpResData, vResData)
		}

		resData = tmpResData
	}

	insertData := make([]*do.NewBinanceTradeHistory, 0)
	// 数据倒序插入，程序走到这里，最多会拉下来初始化：6000，日常：500，最少10条（前边的条件限制）
	for i := len(resData) - 1; i >= 0; i-- {
		insertData = append(insertData, &do.NewBinanceTradeHistory{
			Time:                resData[i].Time,
			Symbol:              resData[i].Symbol,
			Side:                resData[i].Side,
			PositionSide:        resData[i].PositionSide,
			Price:               resData[i].Price,
			Fee:                 resData[i].Fee,
			FeeAsset:            resData[i].FeeAsset,
			Quantity:            resData[i].Quantity,
			QuantityAsset:       resData[i].QuantityAsset,
			RealizedProfit:      resData[i].RealizedProfit,
			RealizedProfitAsset: resData[i].RealizedProfitAsset,
			BaseAsset:           resData[i].BaseAsset,
			Qty:                 resData[i].Qty,
			ActiveBuy:           resData[i].ActiveBuy,
		})
	}

	// 入库
	if 0 >= len(insertData) {
		return nil
	}

	// 推入下单队列
	// todo 这种队列的方式可能存在生产者或消费者出现问题，而丢单的情况，可以考虑更换复杂的方式，即使丢单开不起来会影响开单，关单的话少关单，有仓位检测的二重保障
	pushDataMap := make(map[string]*binanceTrade, 0)
	pushData := make([]*binanceTrade, 0)

	// 代币
	for _, vInsertData := range insertData {
		// 代币，仓位，方向，同一秒 暂时看作一次下单
		timeTmp := vInsertData.Time.(uint64)
		if _, ok := pushDataMap[vInsertData.Symbol.(string)+vInsertData.PositionSide.(string)+vInsertData.Side.(string)+strconv.FormatUint(timeTmp, 10)]; !ok {
			pushDataMap[vInsertData.Symbol.(string)+vInsertData.PositionSide.(string)+vInsertData.Side.(string)+strconv.FormatUint(timeTmp, 10)] = &binanceTrade{
				TraderNum: strconv.FormatUint(traderNum, 10),
				Type:      vInsertData.PositionSide.(string),
				Symbol:    vInsertData.Symbol.(string),
				Side:      vInsertData.Side.(string),
				Position:  "",
				Qty:       "",
				QtyFloat:  vInsertData.Qty.(float64),
				Time:      timeTmp,
			}
		} else { // 到这里一定存在了，累加
			pushDataMap[vInsertData.Symbol.(string)+vInsertData.PositionSide.(string)+vInsertData.Side.(string)+strconv.FormatUint(timeTmp, 10)].QtyFloat += vInsertData.Qty.(float64)
		}
	}

	if 0 < len(pushDataMap) {
		for _, vPushDataMap := range pushDataMap {
			pushData = append(pushData, vPushDataMap)
		}

		if 0 < len(pushData) {
			// 排序，时间靠前的在前边处理
			sort.Slice(pushData, func(i, j int) bool {
				return pushData[i].Time < pushData[j].Time
			})
		}
	}

	err = g.DB().Transaction(context.TODO(), func(ctx context.Context, tx gdb.TX) error {
		// 日常更新数据
		if 0 < len(pushData) {
			// 先查更新仓位，代币，仓位，方向归集好
			for _, vPushDataMap := range pushData {
				// 查询最新未关仓仓位
				var (
					selectOne []*entity.NewBinancePositionHistory
				)
				err = tx.Ctx(ctx).Model("new_binance_position_"+strconv.FormatUint(traderNum, 10)+"_history").
					Where("symbol=?", vPushDataMap.Symbol).Where("side=?", vPushDataMap.Type).Where("opened<=?", vPushDataMap.Time).Where("closed=?", 0).Where("qty>?", 0).
					OrderDesc("id").Limit(1).Scan(&selectOne)
				if err != nil {
					return err
				}

				if 0 >= len(selectOne) {
					// 新增仓位
					if ("LONG" == vPushDataMap.Type && "BUY" == vPushDataMap.Side) ||
						("SHORT" == vPushDataMap.Type && "SELL" == vPushDataMap.Side) {
						// 开仓
						_, err = tx.Ctx(ctx).Insert("new_binance_position_"+strconv.FormatUint(traderNum, 10)+"_history", &do.NewBinancePositionHistory{
							Closed: 0,
							Opened: vPushDataMap.Time,
							Symbol: vPushDataMap.Symbol,
							Side:   vPushDataMap.Type,
							Status: "",
							Qty:    vPushDataMap.QtyFloat,
						})
						if err != nil {
							return err
						}
					}
				} else {
					// 修改仓位
					if ("LONG" == vPushDataMap.Type && "SELL" == vPushDataMap.Side) ||
						("SHORT" == vPushDataMap.Type && "BUY" == vPushDataMap.Side) {
						// 平空 || 平多
						var (
							updateData g.Map
						)

						// todo bsc最高精度小数点7位，到了6位的情况非常少，没意义，几乎等于完全平仓
						if lessThanOrEqualZero(selectOne[0].Qty, vPushDataMap.QtyFloat, 1e-6) {
							updateData = g.Map{
								"qty":    0,
								"closed": gtime.Now().UnixMilli(),
							}
						} else {
							updateData = g.Map{
								"qty": &gdb.Counter{
									Field: "qty",
									Value: -vPushDataMap.QtyFloat, // 加 -值
								},
							}
						}

						_, err = tx.Ctx(ctx).Update("new_binance_position_"+strconv.FormatUint(traderNum, 10)+"_history", updateData, "id", selectOne[0].Id)
						if nil != err {
							return err
						}

					} else if ("LONG" == vPushDataMap.Type && "BUY" == vPushDataMap.Side) ||
						("SHORT" == vPushDataMap.Type && "SELL" == vPushDataMap.Side) {
						// 开多 || 开空
						updateData := g.Map{
							"qty": &gdb.Counter{
								Field: "qty",
								Value: vPushDataMap.QtyFloat,
							},
						}

						_, err = tx.Ctx(ctx).Update("new_binance_position_"+strconv.FormatUint(traderNum, 10)+"_history", updateData, "id", selectOne[0].Id)
						if nil != err {
							return err
						}
					}
				}
			}
		}

		batchSize := 500
		for i := 0; i < len(insertData); i += batchSize {
			end := i + batchSize
			if end > len(insertData) {
				end = len(insertData)
			}
			batch := insertData[i:end]

			_, err = tx.Ctx(ctx).Insert("new_binance_trade_"+strconv.FormatUint(traderNum, 10)+"_history", batch)
			if err != nil {
				return err
			}
		}

		return nil
	})
	if nil != err {
		return err
	}

	// 推入下单队列
	// todo 这种队列的方式可能存在生产者或消费者出现问题，而丢单的情况，可以考虑更换复杂的方式，即使丢单开不起来会影响开单，关单的话少关单，有仓位检测的二重保障
	if !initPull && 0 < len(pushData) {
		s.orderQueue.Push(pushData)
	}

	return nil
}

func (s *sBinanceTraderHistory) ListenThenOrder(ctx context.Context) {
	// 消费者，不停读取队列数据并输出到终端
	var (
		err error
	)
	consumerPool := grpool.New()
	for {
		var (
			dataInterface interface{}
			data          []*binanceTrade
			ok            bool
		)
		if dataInterface = s.orderQueue.Pop(); dataInterface == nil {
			continue
		}

		if data, ok = dataInterface.([]*binanceTrade); !ok {
			// 处理协程
			fmt.Println("监听程序，解析队列数据错误：", dataInterface)
			continue
		}

		// 处理协程
		err = consumerPool.Add(ctx, func(ctx context.Context) {
			for _, v := range data {
				fmt.Println(v)
			}
		})

		if nil != err {
			fmt.Println(err)
		}
	}

}

func (s *sBinanceTraderHistory) pullAndSetHandle(ctx context.Context, traderNum uint64, CountPage int, ipOrderAsc bool, ipMapNeedWait map[string]bool) (resData []*entity.NewBinanceTradeHistory, err error) {
	var (
		PerPullPerPageCountLimitMax = 50 // 每次并行拉取每页最大条数
	)

	if 0 >= s.ips.Size() {
		fmt.Println("ip池子不足，目前数量：", s.ips.Size())
		return nil, err
	}

	// 定义协程共享数据
	dataMap := gmap.New(true) // 结果map，key表示页数，并发安全
	defer dataMap.Clear()
	ipsQueue := gqueue.New() // ip通道，首次使用只用一次
	defer ipsQueue.Close()
	ipsQueueNeedWait := gqueue.New() // ip通道需要等待的
	defer ipsQueueNeedWait.Close()

	// 这里注意一定是key是从0开始到size-1的
	if ipOrderAsc {
		for i := 0; i < s.ips.Size(); i++ {
			if _, ok := ipMapNeedWait[s.ips.Get(i)]; ok {
				if 0 < len(s.ips.Get(i)) { // 1页对应的代理ip的map的key是0
					ipsQueueNeedWait.Push(s.ips.Get(i))
					//if 3949214983441029120 == traderNum {
					//	fmt.Println("暂时别用的ip", s.ips.Get(i))
					//}
				}
			} else {
				if 0 < len(s.ips.Get(i)) { // 1页对应的代理ip的map的key是0
					ipsQueue.Push(s.ips.Get(i))
				}
			}
		}
	} else {
		for i := s.ips.Size() - 1; i >= 0; i-- {
			if _, ok := ipMapNeedWait[s.ips.Get(i)]; ok {
				if 0 < len(s.ips.Get(i)) { // 1页对应的代理ip的map的key是0
					ipsQueueNeedWait.Push(s.ips.Get(i))
					//if 3949214983441029120 == traderNum {
					//	fmt.Println("暂时别用的ip", s.ips.Get(i))
					//}
				}
			} else {
				if 0 < len(s.ips.Get(i)) { // 1页对应的代理ip的map的key是0
					ipsQueue.Push(s.ips.Get(i))
				}
			}
		}
	}

	wg := sync.WaitGroup{}
	for i := 1; i <= CountPage; i++ {
		tmpI := i // go1.22以前的循环陷阱

		wg.Add(1)
		err = s.pool.Add(ctx, func(ctx context.Context) {
			defer wg.Done()

			var (
				retry               = false
				retryTimes          = 0
				retryTimesLimit     = 5 // 重试次数
				successPull         bool
				binanceTradeHistory []*binanceTradeHistoryDataList
			)

			for retryTimes < retryTimesLimit { // 最大重试
				var tmpProxy string
				if 0 < ipsQueue.Len() { // 有剩余先用剩余比较快，不用等2s
					if v := ipsQueue.Pop(); nil != v {
						tmpProxy = v.(string)
						//if 3949214983441029120 == traderNum {
						//	fmt.Println("直接开始", tmpProxy)
						//}
					}
				}

				// 如果没拿到剩余池子
				if 0 >= len(tmpProxy) {
					select {
					case queueItem2 := <-ipsQueueNeedWait.C: // 可用ip，阻塞
						tmpProxy = queueItem2.(string)
						//if 3949214983441029120 == traderNum {
						//	fmt.Println("等待", tmpProxy)
						//}
						time.Sleep(time.Second * 2)
					case <-time.After(time.Minute * 8): // 即使1个ip轮流用，120次查询2秒一次，8分钟超时足够
						fmt.Println("timeout, exit loop")
						break
					}
				}

				// 拿到了代理，执行
				if 0 < len(tmpProxy) {
					binanceTradeHistory, retry, err = s.requestProxyBinanceTradeHistory(tmpProxy, int64(tmpI), int64(PerPullPerPageCountLimitMax), traderNum)
					if nil != err {
						//fmt.Println(err)
					}

					// 使用过的，释放，推入等待queue
					ipsQueueNeedWait.Push(tmpProxy)
				}

				// 需要重试
				if retry {
					//if 3949214983441029120 == traderNum {
					//	fmt.Println("异常需要重试", tmpProxy)
					//}

					retryTimes++
					continue
				}

				// 设置数据
				successPull = true
				dataMap.Set(tmpI, binanceTradeHistory)
				break // 成功直接结束
			}

			// 如果重试次数超过限制且没有成功，存入标记值
			if !successPull {
				dataMap.Set(tmpI, "ERROR")
			}
		})

		if nil != err {
			fmt.Println("添加任务，拉取数据协程异常，错误信息：", err, "交易员：", traderNum)
		}
	}

	// 回收协程
	wg.Wait()

	// 结果，解析处理
	resData = make([]*entity.NewBinanceTradeHistory, 0)
	for i := 1; i <= CountPage; i++ {
		if dataMap.Contains(i) {
			// 从dataMap中获取该页的数据
			dataInterface := dataMap.Get(i)

			// 检查是否是标记值
			if "ERROR" == dataInterface {
				fmt.Println("数据拉取失败，页数：", i, "交易员：", traderNum)
				return nil, err
			}

			// 类型断言，确保dataInterface是我们期望的类型
			if data, ok := dataInterface.([]*binanceTradeHistoryDataList); ok {
				// 现在data是一个binanceTradeHistoryDataList对象数组
				for _, item := range data {
					// 类型处理
					tmpActiveBuy := "false"
					if item.ActiveBuy {
						tmpActiveBuy = "true"
					}

					if "LONG" != item.PositionSide && "SHORT" != item.PositionSide {
						fmt.Println("不识别的仓位，页数：", i, "交易员：", traderNum)
					}

					if "BUY" != item.Side && "SELL" != item.Side {
						fmt.Println("不识别的方向，页数：", i, "交易员：", traderNum)
					}

					resData = append(resData, &entity.NewBinanceTradeHistory{
						Time:                item.Time,
						Symbol:              item.Symbol,
						Side:                item.Side,
						PositionSide:        item.PositionSide,
						Price:               item.Price,
						Fee:                 item.Fee,
						FeeAsset:            item.FeeAsset,
						Quantity:            item.Quantity,
						QuantityAsset:       item.QuantityAsset,
						RealizedProfit:      item.RealizedProfit,
						RealizedProfitAsset: item.RealizedProfitAsset,
						BaseAsset:           item.BaseAsset,
						Qty:                 item.Qty,
						ActiveBuy:           tmpActiveBuy,
					})
				}
			} else {
				fmt.Println("类型断言失败，无法还原数据，页数：", i, "交易员：", traderNum)
				return nil, err
			}
		} else {
			fmt.Println("dataMap不包含，页数：", i, "交易员：", traderNum)
			return nil, err
		}
	}

	return resData, nil
}

func (s *sBinanceTraderHistory) compareBinanceTradeHistoryPageOne(compareMax int64, traderNum uint64, binanceTradeHistoryNewestGroup []*entity.NewBinanceTradeHistory) (ipMapNeedWait map[string]bool, newData []*binanceTradeHistoryDataList, compareResDiff bool, err error) {
	// 试探开始
	var (
		tryLimit            = 3
		binanceTradeHistory []*binanceTradeHistoryDataList
	)

	ipMapNeedWait = make(map[string]bool, 0)
	newData = make([]*binanceTradeHistoryDataList, 0)
	for i := 1; i <= tryLimit; i++ {
		if 0 < s.ips.Size() { // 代理是否不为空
			var (
				ok    = false
				retry = true
			)

			// todo 因为是map，遍历时的第一次，可能一直会用某一条代理信息
			s.ips.Iterator(func(k int, v string) bool {
				ipMapNeedWait[v] = true
				binanceTradeHistory, retry, err = s.requestProxyBinanceTradeHistory(v, 1, compareMax, traderNum)
				if nil != err {
					return true
				}

				if retry {
					return true
				}

				ok = true
				return false
			})

			if ok {
				break
			}
		} else {
			fmt.Println("ip无可用")
			//binanceTradeHistory, err = s.requestBinanceTradeHistory(1, compareMax, traderNum)
			//if nil != err {
			//	return false, err
			//}
			return ipMapNeedWait, newData, false, nil
		}
	}

	// 对比
	if len(binanceTradeHistory) != len(binanceTradeHistoryNewestGroup) {
		fmt.Println("无法对比，条数不同", len(binanceTradeHistory), len(binanceTradeHistoryNewestGroup))
		return ipMapNeedWait, newData, false, nil
	}
	for k, vBinanceTradeHistory := range binanceTradeHistory {
		newData = append(newData, vBinanceTradeHistory)
		if vBinanceTradeHistory.Time == binanceTradeHistoryNewestGroup[k].Time &&
			vBinanceTradeHistory.Symbol == binanceTradeHistoryNewestGroup[k].Symbol &&
			vBinanceTradeHistory.Side == binanceTradeHistoryNewestGroup[k].Side &&
			vBinanceTradeHistory.PositionSide == binanceTradeHistoryNewestGroup[k].PositionSide &&
			IsEqual(vBinanceTradeHistory.Qty, binanceTradeHistoryNewestGroup[k].Qty) && // 数量
			IsEqual(vBinanceTradeHistory.Price, binanceTradeHistoryNewestGroup[k].Price) && //价格
			IsEqual(vBinanceTradeHistory.RealizedProfit, binanceTradeHistoryNewestGroup[k].RealizedProfit) &&
			IsEqual(vBinanceTradeHistory.Quantity, binanceTradeHistoryNewestGroup[k].Quantity) &&
			IsEqual(vBinanceTradeHistory.Fee, binanceTradeHistoryNewestGroup[k].Fee) {
		} else {
			compareResDiff = true
			break
		}
	}

	return ipMapNeedWait, newData, compareResDiff, err
}

type binanceTradeHistoryResp struct {
	Data *binanceTradeHistoryData
}

type binanceTradeHistoryData struct {
	Total uint64
	List  []*binanceTradeHistoryDataList
}

type binanceTradeHistoryDataList struct {
	Time                uint64
	Symbol              string
	Side                string
	Price               float64
	Fee                 float64
	FeeAsset            string
	Quantity            float64
	QuantityAsset       string
	RealizedProfit      float64
	RealizedProfitAsset string
	BaseAsset           string
	Qty                 float64
	PositionSide        string
	ActiveBuy           bool
}

type binanceTrade struct {
	TraderNum string
	Time      uint64
	Symbol    string
	Type      string
	Position  string
	Side      string
	Price     string
	Qty       string
	QtyFloat  float64
}

func (s *sBinanceTraderHistory) requestBinanceTradeHistory(pageNumber int64, pageSize int64, portfolioId uint64) ([]*binanceTradeHistoryDataList, error) {
	var (
		resp   *http.Response
		res    []*binanceTradeHistoryDataList
		b      []byte
		err    error
		apiUrl = "https://www.binance.com/bapi/futures/v1/friendly/future/copy-trade/lead-portfolio/trade-history"
	)

	// 构造请求
	contentType := "application/json"
	data := `{"pageNumber":` + strconv.FormatInt(pageNumber, 10) + `,"pageSize":` + strconv.FormatInt(pageSize, 10) + `,portfolioId:` + strconv.FormatUint(portfolioId, 10) + `}`
	resp, err = http.Post(apiUrl, contentType, strings.NewReader(data))
	if err != nil {
		return nil, err
	}

	// 结果
	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			fmt.Println(err)
		}
	}(resp.Body)

	b, err = ioutil.ReadAll(resp.Body)
	//fmt.Println(string(b))
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	var l *binanceTradeHistoryResp
	err = json.Unmarshal(b, &l)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	if nil == l.Data {
		return res, nil
	}

	if nil == l.Data.List {
		return res, nil
	}

	res = make([]*binanceTradeHistoryDataList, 0)
	for _, v := range l.Data.List {
		res = append(res, v)
	}

	return res, nil
}

func (s *sBinanceTraderHistory) requestProxyBinanceTradeHistory(proxyAddr string, pageNumber int64, pageSize int64, portfolioId uint64) ([]*binanceTradeHistoryDataList, bool, error) {
	var (
		resp   *http.Response
		res    []*binanceTradeHistoryDataList
		b      []byte
		err    error
		apiUrl = "https://www.binance.com/bapi/futures/v1/friendly/future/copy-trade/lead-portfolio/trade-history"
	)

	proxy, err := url.Parse(proxyAddr)
	if err != nil {
		fmt.Println(err)
		return nil, true, err
	}
	netTransport := &http.Transport{
		Proxy:                 http.ProxyURL(proxy),
		MaxIdleConnsPerHost:   10,
		ResponseHeaderTimeout: time.Second * time.Duration(5),
	}
	httpClient := &http.Client{
		Timeout:   time.Second * 10,
		Transport: netTransport,
	}

	// 构造请求
	contentType := "application/json"
	data := `{"pageNumber":` + strconv.FormatInt(pageNumber, 10) + `,"pageSize":` + strconv.FormatInt(pageSize, 10) + `,portfolioId:` + strconv.FormatUint(portfolioId, 10) + `}`
	resp, err = httpClient.Post(apiUrl, contentType, strings.NewReader(data))
	if err != nil {
		fmt.Println(333, err)
		return nil, true, err
	}

	// 结果
	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			fmt.Println(222, err)
		}
	}(resp.Body)

	b, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(111, err)
		return nil, true, err
	}

	var l *binanceTradeHistoryResp
	err = json.Unmarshal(b, &l)
	if err != nil {
		return nil, true, err
	}

	if nil == l.Data {
		return res, true, nil
	}

	res = make([]*binanceTradeHistoryDataList, 0)
	if nil == l.Data.List {
		return res, false, nil
	}

	res = make([]*binanceTradeHistoryDataList, 0)
	for _, v := range l.Data.List {
		res = append(res, v)
	}

	return res, false, nil
}

func (s *sBinanceTraderHistory) requestOrder(pageNumber int64, pageSize int64, portfolioId uint64) ([]*binanceTradeHistoryDataList, error) {
	var (
		resp   *http.Response
		res    []*binanceTradeHistoryDataList
		b      []byte
		err    error
		apiUrl = "https://www.binance.com/bapi/futures/v1/friendly/future/copy-trade/lead-portfolio/trade-history"
	)

	// 构造请求
	contentType := "application/json"
	data := `{"pageNumber":` + strconv.FormatInt(pageNumber, 10) + `,"pageSize":` + strconv.FormatInt(pageSize, 10) + `,portfolioId:` + strconv.FormatUint(portfolioId, 10) + `}`
	resp, err = http.Post(apiUrl, contentType, strings.NewReader(data))
	if err != nil {
		return nil, err
	}

	// 结果
	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			fmt.Println(err)
		}
	}(resp.Body)

	b, err = ioutil.ReadAll(resp.Body)
	//fmt.Println(string(b))
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	var l *binanceTradeHistoryResp
	err = json.Unmarshal(b, &l)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	if nil == l.Data {
		return res, nil
	}

	if nil == l.Data.List {
		return res, nil
	}

	res = make([]*binanceTradeHistoryDataList, 0)
	for _, v := range l.Data.List {
		res = append(res, v)
	}

	return res, nil
}
