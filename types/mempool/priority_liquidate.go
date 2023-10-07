package mempool

import (
	"context"
	"math"

	"github.com/huandu/skiplist"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

var (
	_ Mempool  = (*PriorityNonceMempool[int64])(nil)
	_ Iterator = (*PriorityNonceIterator[int64])(nil)
)

type LiquidationTxKey struct {
	IsLiquidation bool
	Nonce         int64
}

func newLiquidityTxPriority(liquidationMsgTypeURL string) TxPriority[LiquidationTxKey] {
	return TxPriority[LiquidationTxKey]{
		GetTxPriority: func(goCtx context.Context, tx sdk.Tx) LiquidationTxKey {
			var isLiquidation bool
			msgs := tx.GetMsgs()
			if len(msgs) == 1 {
				isLiquidation = sdk.MsgTypeURL(msgs[0]) == liquidationMsgTypeURL
			}
			return LiquidationTxKey{isLiquidation, sdk.UnwrapSDKContext(goCtx).Priority()}
		},
		Compare: func(a, b LiquidationTxKey) int {

			if a.IsLiquidation != b.IsLiquidation {
				if a.IsLiquidation {
					return 1
				}
				return -1
			}
			return skiplist.Int64.Compare(a, b)
		},
		MinValue: LiquidationTxKey{false, math.MinInt64},
	}
}

// DefaultPriorityMempool returns a priorityNonceMempool with no options.
func NewLiquidatorMempool(liquidationMsgTypeURL string) *PriorityNonceMempool[LiquidationTxKey] {
	cfg := PriorityNonceMempoolConfig[LiquidationTxKey]{
		TxPriority: newLiquidityTxPriority(liquidationMsgTypeURL),
	}
	return NewPriorityMempool(cfg)
}
