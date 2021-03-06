package backtest

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jujili/clock"
	"github.com/jujili/exch"
)

// BalanceService 会在每天的凌晨零点零分零秒记录 balance 的总价值
// prices 里面需要放好各种资产的价格，不要忘记 capital 的价格是 1
func BalanceService(ctx context.Context, ps Pubsub, prices map[string]float64, asset string) {
	log.Println("进入 BalanceService...")
	ticks, err := ps.Subscribe(ctx, "tick")
	if err != nil {
		panic(err)
	}
	decTick := exch.DecTickFunc()
	//
	balances, err := ps.Subscribe(ctx, "balance")
	if err != nil {
		panic(err)
	}
	decBal := exch.DecBalanceFunc()
	go func() {
		log.Println("进入 BalanceService go func ...")
		// 创建模拟 clock
		msg := <-ticks
		tick := decTick(msg.Payload)
		msg.Ack()
		prices[asset] = tick.Price
		clock := clock.NewSimulator(tick.Date)
		everyNewDay := clock.EveryDay(0, 0, 0)
		// 另起一个 goroutine，更新 clock
		go func() {
			log.Println("进入 BalanceService 时钟 goroutine ...")
			tks, _ := ps.Subscribe(ctx, "tick")
			for msg := range tks {
				tick := decTick(msg.Payload)
				msg.Ack()
				clock.SetOrPanic(tick.Date)
				// log.Println("将 BalanceService 的本地始终设置成了", tick.Date)
			}
			log.Println("balance service, ticks end, not update clock")
		}()
		//
		go func() {
			log.Println("进入 BalanceService 帐户记录 goroutine ...")
			var bal *exch.Balance
			bs := make([]balanceSnap, 0, 2048)
			count := 0
			for count < 2 {
				// log.Println("进入 BalanceService 帐户记录 for ...")
				select {
				case <-ctx.Done():
					log.Fatalln("BalanceService Down: ", ctx.Err())
				case msg, ok := <-ticks:
					if !ok {
						count++
						ticks = nil
						continue
					}
					tick := decTick(msg.Payload)
					msg.Ack()
					prices[asset] = tick.Price
				case msg, ok := <-balances:
					if !ok {
						count++
						balances = nil
						continue
					}
					bal = decBal(msg.Payload)
					msg.Ack()
				case date := <-everyNewDay:
					newBal := newBalanceSnap(date, bal, prices, asset)
					bs = append(bs, newBal)
					log.Println("\t", date, bal, prices, newBal)
				}
			}
			log.Println("all balance snap is")
			log.Println(bs)
		}()
	}()
}

type balanceSnap struct {
	date   time.Time
	amount float64
	price  float64
}

func newBalanceSnap(date time.Time, balance *exch.Balance, prices map[string]float64, asset string) balanceSnap {
	return balanceSnap{
		date:   date,
		amount: balance.Total(prices),
		price:  prices[asset],
	}
}

func (bs balanceSnap) String() string {
	return fmt.Sprintf("%s, amount, %f, price, %f\n", bs.date, bs.amount, bs.price)
}
