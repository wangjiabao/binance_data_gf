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
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type (
	sBinanceTraderHistory struct {
		pool *grpool.Pool
		// 针对binance拉取交易员下单接口定制的数据结构，每页数据即使同一个ip不同的交易员是可以返回数据，在逻辑上和这个设计方式要达成共识再设计后续的程序逻辑。
		// 考虑实际的业务场景，接口最多6000条，每次查50条，最多需要120个槽位。假设，每10个槽位一循环，意味着对应同一个交易员每次并行使用10个ip查询10页数据。
		//ips map[int]*proxyData
		ips *gmap.IntStrMap
	}
)

func init() {
	service.RegisterBinanceTraderHistory(New())
}

func New() *sBinanceTraderHistory {
	return &sBinanceTraderHistory{
		grpool.New(), // 这里是请求协程池子，可以配合着可并行请求binance的限制使用，来限制最大共存数，后续jobs都将排队，考虑到上层的定时任务
		gmap.NewIntStrMap(true),
	}
}

func IsEqual(f1, f2 float64) bool {
	if f1 > f2 {
		return f1-f2 < 0.000000001
	} else {
		return f2-f1 < 0.000000001
	}
}

type proxyData struct {
	Ip   string
	Port int64
}

type proxyRep struct {
	Data []*proxyData
}

// 拉取代理列表
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
	var (
		res []*proxyData
	)
	for i := 0; i < 1000; i++ {
		res, err = requestProxy()
		if nil != err {
			fmt.Println("ip池子更新出错", err)
			continue
		}

		if 0 < len(res) {
			break
		}

		time.Sleep(time.Second * 1)
	}

	s.ips.Clear()

	// 更新
	for k, v := range res {
		s.ips.Set(k, "http://"+v.Ip+":"+strconv.FormatInt(v.Port, 10)+"/")
	}

	return nil
}

func (s *sBinanceTraderHistory) PullAndOrder(ctx context.Context, traderNum uint64) (err error) {
	start := time.Now()
	defer func() {
		fmt.Printf("程序运行时长: %v\n", time.Since(start))
	}()

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
		compareMax                     = 10 // 预设最大对比条数，小于最大限制10条
		currentCompareMax              int  // 实际获得对比条数
		binanceTradeHistoryNewestGroup []*entity.NewBinanceTradeHistory
		resData                        []*entity.NewBinanceTradeHistory
		initPull                       bool
	)

	err = g.Model("new_binance_trade_" + strconv.FormatUint(traderNum, 10) + "_history").Ctx(ctx).Limit(compareMax).OrderDesc("id").Scan(&binanceTradeHistoryNewestGroup)
	if nil != err {
		return err
	}

	currentCompareMax = len(binanceTradeHistoryNewestGroup)

	insertData := make([]*do.NewBinanceTradeHistory, 0)
	// 数据库无数据，拉取满额6000条数据
	if 0 >= currentCompareMax {
		initPull = true
		resData, err = s.pullAndSetHandle(ctx, traderNum, 120) // 执行
		if nil != err {
			fmt.Println("初始化，执行拉取数据协程异常，错误信息：", err, "交易员：", traderNum)
		}

		if 0 >= len(resData) {
			fmt.Println("初始化，执行拉取数据协程异常，空数据：", len(resData), "交易员：", traderNum)
			return nil
		}

	} else if compareMax <= currentCompareMax {
		// 试探开始
		var (
			compareResDiff bool
		)
		compareResDiff, err = s.compareBinanceTradeHistoryPageOne(int64(compareMax), traderNum, binanceTradeHistoryNewestGroup)
		if nil != err {
			return err
		}

		// 相同，返回
		if !compareResDiff {
			return nil
		}

		// 不同，开始捕获
		resData, err = s.pullAndSetHandle(ctx, traderNum, 10) // todo 执行，目前猜测最大500条，根据经验拍脑袋
		if nil != err {
			fmt.Println("日常，执行拉取数据协程异常，错误信息：", err, "交易员：", traderNum)
		}

		if 0 >= len(resData) {
			return nil
		}

	} else {
		fmt.Println("执行拉取数据协程异常，查询数据库数据条数不在范围：", compareMax, currentCompareMax, "交易员：", traderNum, "初始化：", initPull)
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
		fmt.Println("执行拉取数据协程异常，条数不足，条数：", len(resData), "交易员：", traderNum, "初始化：", initPull)
		return nil
	}

	if 0 >= len(afterCompare) {
		fmt.Println("执行拉取数据协程异常，条数不足，条数：", len(resData), "交易员：", traderNum, "初始化：", initPull)
		return nil
	}

	compareResDiff, err = s.compareBinanceTradeHistoryPageOne(int64(compareMax), traderNum, afterCompare)
	if nil != err {
		return err
	}

	// 不同，则拉取数据的时间有新单，放弃操作，等待下次执行
	if compareResDiff {
		fmt.Println("执行拉取数据协程异常，有新单", "交易员：", traderNum, "初始化：", initPull)
		return nil
	}

	// 非初始化，截断数据
	if !initPull {
		tmpResData := make([]*entity.NewBinanceTradeHistory, 0)
		tmpCurrentCompareMax := currentCompareMax
		fmt.Println("对比：", resData[0], binanceTradeHistoryNewestGroup[0])
		for k, vResData := range resData {

			if (len(resData) - k) <= tmpCurrentCompareMax { // 还剩下几条
				tmpCurrentCompareMax = len(resData) - k
			}

			if k == 2 {
				fmt.Println(vResData)
			}
			tmp := 0
			if 0 < tmpCurrentCompareMax {
				for i := 0; i < tmpCurrentCompareMax; i++ { // todo 如果只剩下最大条数以内的数字，只能兼容着比较，这里根据经验判断会不会出现吧
					if k == 2 {
						fmt.Println(resData[k+i], binanceTradeHistoryNewestGroup[i])
					}
					if resData[k+i].Time == binanceTradeHistoryNewestGroup[i].Time &&
						resData[k+i].Symbol == binanceTradeHistoryNewestGroup[i].Symbol &&
						resData[k+i].Side == binanceTradeHistoryNewestGroup[i].Side &&
						resData[k+i].PositionSide == binanceTradeHistoryNewestGroup[i].PositionSide &&
						IsEqual(resData[k+i].Qty, binanceTradeHistoryNewestGroup[i].Qty) && // 数量
						IsEqual(resData[k+i].Price, binanceTradeHistoryNewestGroup[i].Price) && //价格
						IsEqual(resData[k+i].RealizedProfit, binanceTradeHistoryNewestGroup[i].RealizedProfit) &&
						IsEqual(resData[k+i].Quantity, binanceTradeHistoryNewestGroup[i].Quantity) &&
						IsEqual(resData[k+i].Fee, binanceTradeHistoryNewestGroup[i].Fee) {
					} else {
						fmt.Println(1111)
						tmp++
					}
				}

				if tmpCurrentCompareMax == tmp {
					break
				}
			} else {
				break
			}

			tmpResData = append(tmpResData, vResData)
			fmt.Println("新增：", vResData)
		}

		resData = tmpResData
	}

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

	err = g.DB().Transaction(context.TODO(), func(ctx context.Context, tx gdb.TX) error {
		_, err = tx.Ctx(ctx).Insert("new_binance_trade_"+strconv.FormatUint(traderNum, 10)+"_history", insertData)
		if err != nil {
			return err
		}

		return nil
	})
	if nil != err {
		return err
	}

	return nil
}

func (s *sBinanceTraderHistory) pullAndSetHandle(ctx context.Context, traderNum uint64, CountPage int) (resData []*entity.NewBinanceTradeHistory, err error) {
	var (
		PerPullPerPageCountLimitMax = 50 // 每次并行拉取每页最大条数
		ipsCount                    = s.ips.Size()
	)

	if 0 >= ipsCount {
		fmt.Println("ip池子不足，目前数量：", ipsCount)
	}

	// 定义协程共享数据
	dataMap := gmap.New(true) // 结果map，key页数，并发安全
	defer dataMap.Clear()
	ipsQueue := gqueue.New() // ip通道
	defer ipsQueue.Close()
	ticker := time.NewTicker(15 * time.Second) // 定时器
	defer ticker.Stop()

	// 当所拥有的ip大于查询页数，直接进入预备状态
	if CountPage < ipsCount {
		for i := CountPage; i < ipsCount; i++ {
			if 0 < len(s.ips.Get(i)) {
				ipsQueue.Push(s.ips.Get(i))
			}
		}
	}

	wg := sync.WaitGroup{}
	for i := 1; i <= CountPage; i++ {
		tmpI := i // go1.22以前的循环陷阱

		wg.Add(1)
		err = s.pool.Add(ctx, func(ctx context.Context) {
			defer wg.Done()

			// 满足并行数，超过ip池子总数时，睡眠3秒后复用ip，度过ip频繁访问的禁用时长
			if tmpI > ipsCount {
				time.Sleep(time.Second * 3 * time.Duration(tmpI/ipsCount)) // 几倍
			}

			ipIndex := (tmpI - 1) % ipsCount // 循环复用

			var (
				tmpProxy            = s.ips.Get(ipIndex)
				retry               = true
				binanceTradeHistory []*binanceTradeHistoryDataList
			)

			for {
				// 执行
				if 0 < len(tmpProxy) {
					binanceTradeHistory, retry, err = s.requestProxyBinanceTradeHistory(tmpProxy, int64(tmpI), int64(PerPullPerPageCountLimitMax), traderNum)
					if nil != err {
						fmt.Println(err)
					}
				}

				// 需要重试
				if retry {
					select {
					case queueItem := <-ipsQueue.C: // 可用ip，阻塞
						tmpProxy = queueItem.(string)
						time.Sleep(time.Second * 3) // 这里一定刚用ip不久，间隔3秒
					case <-ticker.C: // 定时器，阻塞超过一定时间，且没有可用ip，将全部ip重新推入
						s.ips.Iterator(func(k int, v string) bool {
							if 0 == k {
								tmpProxy = v
							}
							ipsQueue.Push(v)
							return true
						})

					case <-time.After(time.Minute * 10): // 即使1个ip轮流用，120次查询3秒一次，10分钟超时
						fmt.Println("timeout, exit loop")
						break
					}

					continue
				} else {
					//fmt.Println("可用的proxy", tmpProxy)
					// 直接获取到数据，ip可用性可以
					ipsQueue.Push(tmpProxy)
				}

				break
			}

			// 设置数据
			dataMap.Set(tmpI, binanceTradeHistory)
		})

		if nil != err {
			fmt.Println("添加任务，拉取数据协程异常，错误信息：", err, "交易员：", traderNum)
		}
	}

	// 回收协程
	wg.Wait()

	// 结果数据
	resData = make([]*entity.NewBinanceTradeHistory, 0)
	for i := 1; i <= CountPage; i++ {
		if dataMap.Contains(i) {
			// 从dataMap中获取该页的数据
			dataInterface := dataMap.Get(i)

			// 类型断言，确保dataInterface是我们期望的类型
			if data, ok := dataInterface.([]*binanceTradeHistoryDataList); ok {
				// 现在data是一个binanceTradeHistoryDataList对象数组
				for _, item := range data {

					// 类型处理
					tmpActiveBuy := "false"
					if item.ActiveBuy {
						tmpActiveBuy = "true"
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
				fmt.Println("类型断言失败，无法还原数据")
			}
		} else {
			fmt.Printf("dataMap不包含页数: %d 的数据\n", i)
		}
	}

	return resData, nil
}

func (s *sBinanceTraderHistory) compareBinanceTradeHistoryPageOne(compareMax int64, traderNum uint64, binanceTradeHistoryNewestGroup []*entity.NewBinanceTradeHistory) (compareResDiff bool, err error) {
	// 试探开始
	var (
		binanceTradeHistory []*binanceTradeHistoryDataList
	)
	if 0 < s.ips.Size() { // 代理是否不为空
		var (
			retry = true
		)

		s.ips.Iterator(func(k int, v string) bool {
			binanceTradeHistory, retry, err = s.requestProxyBinanceTradeHistory(v, 1, compareMax, traderNum)
			if nil != err {
				fmt.Println(err)
				return true
			}

			if retry {
				return true
			}

			return false
		})

	} else {
		binanceTradeHistory, err = s.requestBinanceTradeHistory(1, compareMax, traderNum)
		if nil != err {
			return false, err
		}
	}

	// 对比
	if len(binanceTradeHistory) != len(binanceTradeHistoryNewestGroup) {
		fmt.Println("无法对比，条数不同", len(binanceTradeHistory), len(binanceTradeHistoryNewestGroup))
		return true, nil
	}
	for k, vBinanceTradeHistory := range binanceTradeHistory {
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

	return compareResDiff, err
}

func (s *sBinanceTraderHistory) handleGoroutine(ctx context.Context, tmpPageNumber int64, traderNum uint64) (binanceTradeHistory []*binanceTradeHistoryDataList) {
	var (
		err error
	)

	binanceTradeHistory, err = s.requestBinanceTradeHistory(tmpPageNumber, 50, traderNum)
	if nil != err {
		time.Sleep(2 * time.Second)
		fmt.Println(err)
		return
	}

	if nil == binanceTradeHistory {
		return binanceTradeHistory // 直接返回
	}
	if 0 >= len(binanceTradeHistory) {
		return binanceTradeHistory // 直接返回
	}

	return binanceTradeHistory

	//currentLen := len(binanceTradeHistory)
	//for kPage, vBinanceTradeHistory := range binanceTradeHistory {
	//	// 第1-n条一致认为是已有数据
	//
	//	if 0 == currentCompareMax { // 数据库无数据
	//		// 不做限制
	//	} else { // 有数据且不足对比条数
	//
	//		// 页面上的数据一定大于等于数据库中对比条数
	//		for kDatabase, binanceTradeHistoryNewest := range binanceTradeHistoryNewestGroup {
	//			if vBinanceTradeHistory.Time == binanceTradeHistoryNewest.Time &&
	//				vBinanceTradeHistory.Symbol == binanceTradeHistoryNewest.Symbol &&
	//				vBinanceTradeHistory.Side == binanceTradeHistoryNewest.Side &&
	//				vBinanceTradeHistory.PositionSide == binanceTradeHistoryNewest.PositionSide &&
	//				IsEqual(vBinanceTradeHistory.Qty, binanceTradeHistoryNewest.Qty) && // 数量
	//				IsEqual(vBinanceTradeHistory.Price, binanceTradeHistoryNewest.Price) && //价格
	//				IsEqual(vBinanceTradeHistory.RealizedProfit, binanceTradeHistoryNewest.RealizedProfit) &&
	//				IsEqual(vBinanceTradeHistory.Quantity, binanceTradeHistoryNewest.Quantity) &&
	//				IsEqual(vBinanceTradeHistory.Fee, binanceTradeHistoryNewest.Fee) {
	//
	//			}
	//		}
	//
	//		time.Sleep(2 * time.Second)
	//		last = false // 终止
	//		break
	//	}
	//
	//	// 类型处理
	//	tmpActiveBuy := "false"
	//	if vBinanceTradeHistory.ActiveBuy {
	//		tmpActiveBuy = "true"
	//	}
	//
	//	// 追加
	//	insertBinanceTrade = append(insertBinanceTrade, &BinanceTradeHistory{
	//		Time:                vBinanceTradeHistory.Time,
	//		Symbol:              vBinanceTradeHistory.Symbol,
	//		Side:                vBinanceTradeHistory.Side,
	//		Price:               vBinanceTradeHistory.Price,
	//		Fee:                 vBinanceTradeHistory.Fee,
	//		FeeAsset:            vBinanceTradeHistory.FeeAsset,
	//		Quantity:            vBinanceTradeHistory.Quantity,
	//		QuantityAsset:       vBinanceTradeHistory.QuantityAsset,
	//		RealizedProfit:      vBinanceTradeHistory.RealizedProfit,
	//		RealizedProfitAsset: vBinanceTradeHistory.RealizedProfitAsset,
	//		BaseAsset:           vBinanceTradeHistory.BaseAsset,
	//		Qty:                 vBinanceTradeHistory.Qty,
	//		PositionSide:        vBinanceTradeHistory.PositionSide,
	//		ActiveBuy:           tmpActiveBuy,
	//	})
	//}
	//
	//// 不满50条，查到了最后，一般出现在大于50条的初始化
	//if 50 > len(binanceTradeHistory) {
	//	break
	//}
	//
	//tmpPageNumber++
	//time.Sleep(2 * time.Second)
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
		log.Fatal(err)
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
		return nil, true, err
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
