// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// NewUserBindTraderTwo is the golang structure for table new_user_bind_trader_two.
type NewUserBindTraderTwo struct {
	Id        uint        `json:"id"        orm:"id"         ` // 主键id
	UserId    uint        `json:"userId"    orm:"user_id"    ` // 用户id
	TraderId  uint        `json:"traderId"  orm:"trader_id"  ` // 交易员id
	Amount    uint64      `json:"amount"    orm:"amount"     ` //
	Status    uint        `json:"status"    orm:"status"     ` // 可用0，不可用1，待更换2
	InitOrder uint        `json:"initOrder" orm:"init_order" ` // 绑后是否初始化仓位
	CreatedAt *gtime.Time `json:"createdAt" orm:"created_at" ` // 创建时间
	UpdatedAt *gtime.Time `json:"updatedAt" orm:"updated_at" ` // 更新时间
	Num       float64     `json:"num"       orm:"num"        ` //
}
