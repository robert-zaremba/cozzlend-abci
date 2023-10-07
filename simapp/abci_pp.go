package simapp

import (
	// "bytes"
	// "context"

	"fmt"

	"github.com/cockroachdb/errors"
	abci "github.com/cometbft/cometbft/abci/types"

	// cryptoenc "github.com/cometbft/cometbft/crypto/encoding"
	// cmtprotocrypto "github.com/cometbft/cometbft/proto/tendermint/crypto"
	// cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	// protoio "github.com/cosmos/gogoproto/io"
	// "github.com/cosmos/gogoproto/proto"

	"cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/mempool"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

type LiquidationProposalHandler struct {
	Mempool    mempool.Mempool
	txSelector baseapp.TxSelector

	txDecoder func(txBz []byte) (sdk.Tx, error)
	txEncoder func(tx sdk.Tx) ([]byte, error)

	txVerifier baseapp.ProposalTxVerifier

	priorityMsgType string

	bank bankkeeper.Keeper
}

func NewLiquidationProposalHandler(txVerifier baseapp.ProposalTxVerifier, priorityMsgType string, bank bankkeeper.Keeper) *LiquidationProposalHandler {
	return &LiquidationProposalHandler{
		Mempool:         mempool.NewLiquidatorMempool(priorityMsgType),
		txDecoder:       txVerifier.TxDecode,
		txEncoder:       txVerifier.TxEncode,
		txVerifier:      txVerifier,
		txSelector:      baseapp.NewDefaultTxSelector(),
		priorityMsgType: priorityMsgType,
		bank:            bank,
	}
}

func (h *LiquidationProposalHandler) PrepareProposalHandler(ctx sdk.Context, req *abci.RequestPrepareProposal) (*abci.ResponsePrepareProposal, error) {
	var maxBlockGas uint64
	if b := ctx.ConsensusParams().Block; b != nil {
		maxBlockGas = uint64(b.MaxGas)
	}

	defer h.txSelector.Clear()

	totalLiquidations := math.ZeroInt()

	iterator := h.Mempool.Select(ctx, req.Txs)
	for iterator != nil {
		memTx := iterator.Tx()

		if msgs := memTx.GetMsgs(); len(msgs) == 1 {
			if sdk.MsgTypeURL(msgs[0]) == h.priorityMsgType {
				msg := msgs[0].(*banktypes.MsgLiquidate)
				totalLiquidations = totalLiquidations.Add(msg.Amount.Amount)
			}
		}

		// NOTE: Since transaction verification was already executed in CheckTx,
		// which calls mempool.Insert, in theory everything in the pool should be
		// valid. But some mempool implementations may insert invalid txs, so we
		// check again.
		txBz, err := h.txVerifier.PrepareProposalVerifyTx(memTx)
		if err != nil {
			err := h.Mempool.Remove(memTx)
			if err != nil && !errors.Is(err, mempool.ErrTxNotFound) {
				return nil, err
			}
		} else {
			stop := h.txSelector.SelectTxForProposal(ctx, uint64(req.MaxTxBytes), maxBlockGas, memTx, txBz)
			if stop {
				break
			}
		}

		iterator = iterator.Next()
	}

	h.bank.SetTotalLiquidations(totalLiquidations)
	txs := h.txSelector.SelectedTxs(ctx)
	if !totalLiquidations.IsZero() {
		bz, _ := totalLiquidations.Marshal()
		txs = append([][]byte{bz}, txs...)
	}

	return &abci.ResponsePrepareProposal{Txs: txs}, nil
}

func (h *LiquidationProposalHandler) ProcessProposalHandler(ctx sdk.Context, req *abci.RequestProcessProposal) (*abci.ResponseProcessProposal, error) {
	var totalTxGas uint64

	var maxBlockGas int64
	if b := ctx.ConsensusParams().Block; b != nil {
		maxBlockGas = b.MaxGas
	}

	totalLiquidations := math.ZeroInt()
	expectedTotalLiquidations := math.ZeroInt()
	txs := req.Txs

	if len(txs) > 0 {
		a := math.ZeroInt()
		if a.Unmarshal(txs[0]) == nil {
			txs = txs[1:]
			expectedTotalLiquidations = a
		}
	}

	for _, txBytes := range txs {
		tx, err := h.txVerifier.ProcessProposalVerifyTx(txBytes)
		if err != nil {
			return &abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_REJECT}, nil
		}

		if msgs := tx.GetMsgs(); len(msgs) == 1 {
			if sdk.MsgTypeURL(msgs[0]) == h.priorityMsgType {
				msg := msgs[0].(*banktypes.MsgLiquidate)
				totalLiquidations = totalLiquidations.Add(msg.Amount.Amount)
			}
		}

		if maxBlockGas > 0 {
			gasTx, ok := tx.(baseapp.GasTx)
			if ok {
				totalTxGas += gasTx.GetGas()
			}

			if totalTxGas > uint64(maxBlockGas) {
				return &abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_REJECT}, nil
			}
		}
	}

	if !totalLiquidations.Equal(expectedTotalLiquidations) {
		fmt.Println("================ expected liquidations don't match!!!", totalLiquidations, expectedTotalLiquidations)
		return &abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_REJECT}, nil
	}

	return &abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_ACCEPT}, nil
}
