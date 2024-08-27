package spammer

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"math/big"
	"time"

	op_e2e "github.com/ethereum-optimism/optimism/op-e2e"
	"github.com/ethereum-optimism/optimism/op-e2e/bindings"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/wait"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

const batchSize = 50

func SendTx(sk *ecdsa.PrivateKey, backend *ethclient.Client, to common.Address, value *big.Int) (*types.Transaction, error) {
	sender := crypto.PubkeyToAddress(sk.PublicKey)
	nonce, err := backend.NonceAt(context.Background(), sender, nil)
	if err != nil {
		fmt.Printf("Could not get pending nonce: %v", err)
	}
	return sendTxWithNonce(sk, backend, to, value, nonce)
}

func sendTxWithNonce(sk *ecdsa.PrivateKey, backend *ethclient.Client, to common.Address, value *big.Int, nonce uint64) (*types.Transaction, error) {
	chainid, err := backend.ChainID(context.Background())
	if err != nil {
		return nil, err
	}
	gp, _ := backend.SuggestGasPrice(context.Background())
	gas, _ := backend.EstimateGas(context.Background(), ethereum.CallMsg{
		From:     crypto.PubkeyToAddress(sk.PublicKey),
		To:       &to,
		Gas:      math.MaxUint64,
		GasPrice: gp,
		Value:    value,
		Data:     nil,
	})
	tx := types.NewTransaction(nonce, to, value, gas, gp.Mul(gp, big.NewInt(100)), nil)
	signedTx, _ := types.SignTx(tx, types.NewEIP155Signer(chainid), sk)
	return signedTx, backend.SendTransaction(context.Background(), signedTx)
}

func sendRecurringTx(sk *ecdsa.PrivateKey, backend *ethclient.Client, to common.Address, value *big.Int, numTxs uint64) (*types.Transaction, error) {
	sender := crypto.PubkeyToAddress(sk.PublicKey)
	nonce, err := backend.NonceAt(context.Background(), sender, nil)
	if err != nil {
		return nil, err
	}
	var (
		tx *types.Transaction
	)
	for i := 0; i < int(numTxs); i++ {
		tx, err = sendTxWithNonce(sk, backend, to, value, nonce+uint64(i))
	}
	return tx, err
}

func Unstuck(config *Config) error {
	if err := tryUnstuck(config, config.faucet); err != nil {
		return err
	}
	for _, key := range config.keys {
		if err := tryUnstuck(config, key); err != nil {
			return err
		}
	}
	return nil
}

func tryUnstuck(config *Config, sk *ecdsa.PrivateKey) error {
	var (
		client = ethclient.NewClient(config.l2Backend)
		addr   = crypto.PubkeyToAddress(sk.PublicKey)
	)
	for i := 0; i < 100; i++ {
		noTx, err := isStuck(config, addr)
		if err != nil {
			return err
		}
		if noTx == 0 {
			return nil
		}

		// Self-transfer of 1 wei to unstuck
		if noTx > batchSize {
			noTx = batchSize
		}
		fmt.Println("Sending transaction to unstuck account")
		tx, err := sendRecurringTx(sk, client, addr, big.NewInt(1), noTx)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
		defer cancel()
		if _, err := bind.WaitMined(ctx, client, tx); err != nil {
			return err
		}
	}
	fmt.Printf("Could not unstuck account %v after 100 tries\n", addr)
	return errors.New("unstuck timed out, please retry manually")
}

func isStuck(config *Config, account common.Address) (uint64, error) {
	client := ethclient.NewClient(config.l2Backend)
	nonce, err := client.NonceAt(context.Background(), account, nil)
	if err != nil {
		return 0, err
	}

	pendingNonce, err := client.PendingNonceAt(context.Background(), account)
	if err != nil {
		return 0, err
	}

	if pendingNonce != nonce {
		fmt.Printf("Account %v is stuck: pendingNonce: %v currentNonce: %v, missing nonces: %v\n", account, pendingNonce, nonce, pendingNonce-nonce)
		return pendingNonce - nonce, nil
	}
	return 0, nil
}

// SendDepositTx creates and sends a deposit transaction.
// The L1 transaction, including sender, is configured by the l1Opts param.
// The L2 transaction options can be configured by modifying the DepositTxOps value supplied to applyL2Opts
// Will verify that the transaction is included with the expected status on L1 and L2
// Returns the receipt of the L2 transaction
func SendDepositTx(optimismPortalAddr common.Address, l1Client *ethclient.Client, l2Client *ethclient.Client, l1Opts *bind.TransactOpts, applyL2Opts op_e2e.DepositTxOptsFn) (*types.Receipt, error) {
	fmt.Printf("Sending deposit transaction\n")
	l2Opts := defaultDepositTxOpts(l1Opts)
	applyL2Opts(l2Opts)

	// Find deposit contract
	depositContract, err := bindings.NewOptimismPortal(optimismPortalAddr, l1Client)
	if err != nil {
		return nil, err
	}
	// Finally send TX
	// Add 10% padding for the L1 gas limit because the estimation process can be affected by the 1559 style cost scale
	// for buying L2 gas in the portal contracts.
	gasLimit := uint64(10000000)

	tx, err := depositContract.DepositTransaction(l1Opts, l2Opts.ToAddr, l2Opts.Value, gasLimit, l2Opts.IsCreation, l2Opts.Data)

	if err == nil {
		fmt.Printf("Transaction created successfully:\n")
		fmt.Printf("  Hash: %s\n", tx.Hash().Hex())
		fmt.Printf("  Nonce: %d\n", tx.Nonce())
		fmt.Printf("  Gas Price: %s\n", tx.GasPrice().String())
		fmt.Printf("  Gas: %d\n", tx.Gas())
	}
	if err != nil {
		fmt.Printf("Error sending transaction: %v\n", err)
		return nil, err
	}

	// Wait for transaction on L1
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	l1Receipt, err := wait.ForReceiptOK(ctx, l1Client, tx.Hash())
	fmt.Printf("L1 receipt: %v\n", l1Receipt)
	if err != nil {
		return nil, err
	}

	// Wait for transaction to be included on L2
	reconstructedDep, err := derive.UnmarshalDepositLogEvent(l1Receipt.Logs[0])
	tx = types.NewTx(reconstructedDep)
	l2Receipt, err := wait.ForReceipt(ctx, l2Client, tx.Hash(), l2Opts.ExpectedStatus)
	if err != nil {
		return nil, err
	}

	return l2Receipt, nil
}

func defaultDepositTxOpts(opts *bind.TransactOpts) *op_e2e.DepositTxOpts {
	return &op_e2e.DepositTxOpts{
		ToAddr:         opts.From,
		Value:          opts.Value,
		GasLimit:       1_000_000,
		IsCreation:     false,
		Data:           nil,
		ExpectedStatus: types.ReceiptStatusSuccessful,
	}
}
