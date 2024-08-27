package main

import (
	"fmt"
	"math/big"
	"os"
	"time"

	"github.com/MariusVanDerWijden/tx-fuzz/flags"
	"github.com/MariusVanDerWijden/tx-fuzz/spammer"
	"github.com/ethereum/go-ethereum/params"
	"github.com/urfave/cli/v2"
)

var airdropCommand = &cli.Command{
	Name:   "airdrop",
	Usage:  "Airdrops to a list of accounts",
	Action: runAirdrop,
	Flags: []cli.Flag{
		flags.SkFlag,
		flags.RpcFlag,
	},
}

var spamCommand = &cli.Command{
	Name:   "spam",
	Usage:  "Send spam transactions",
	Action: runBasicSpam,
	Flags:  flags.SpamFlags,
}

var singleSpamCommand = &cli.Command{
	Name:   "singleSpam",
	Usage:  "Send single group of spam transaction",
	Action: runSingleSpam,
	Flags:  flags.SpamFlags,
}

// var singleBlobSpamCommand = &cli.Command{
// 	Name:   "singleBlob",
// 	Usage:  "Send single group of blob transaction",
// 	Action: runSingleBlob,
// 	Flags:  flags.SpamFlags,
// }

// var blobSpamCommand = &cli.Command{
// 	Name:   "blobs",
// 	Usage:  "Send blob spam transactions",
// 	Action: runBlobSpam,
// 	Flags:  flags.SpamFlags,
// }

var createCommand = &cli.Command{
	Name:   "create",
	Usage:  "Create ephemeral accounts",
	Action: runCreate,
	Flags: []cli.Flag{
		flags.CountFlag,
		flags.RpcFlag,
	},
}

var unstuckCommand = &cli.Command{
	Name:   "unstuck",
	Usage:  "Tries to unstuck an account",
	Action: runUnstuck,
	Flags:  flags.SpamFlags,
}

var depositValidTxCommand = &cli.Command{
	Name:   "depositValidTx",
	Usage:  "Send a valid deposit transaction",
	Action: runDepositValidTx,
	Flags:  flags.SpamFlags,
}

func initApp() *cli.App {
	app := cli.NewApp()
	app.Name = "tx-fuzz"
	app.Usage = "Fuzzer for sending spam transactions"
	app.Commands = []*cli.Command{
		airdropCommand,
		spamCommand,
		// blobSpamCommand,
		createCommand,
		unstuckCommand,
		singleSpamCommand,
		// singleBlobSpamCommand,
	}
	return app
}

var app = initApp()

func main() {
	// eth.sendTransaction({from:personal.listAccounts[0], to:"0xb02A2EdA1b317FBd16760128836B0Ac59B560e9D", value: "100000000000000"})
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runAirdrop(c *cli.Context) error {
	config, err := spammer.NewConfigFromContext(c)
	if err != nil {
		return err
	}
	txPerAccount := config.N
	airdropValue := new(big.Int).Mul(big.NewInt(int64(txPerAccount*100000)), big.NewInt(params.GWei))
	spammer.Airdrop(config, airdropValue)
	return nil
}

func spam(config *spammer.Config, spamFn spammer.Spam, airdropValue *big.Int) error {
	// Make sure the accounts are unstuck before sending any transactions

	spammer.Unstuck(config)
	for {
		if err := spammer.Airdrop(config, airdropValue); err != nil {
			fmt.Printf("error in airdrop function, exiting the for loop\n")
			return err
		}
		spammer.SpamTransactions(config, spamFn)
		time.Sleep(time.Duration(config.SlotTime) * time.Second)
	}
}
func singleSpam(config *spammer.Config, airdropValue *big.Int) error {
	// Check for specific invalid transaction flags
	if config.InvalidGas {
		// works fine, sends invalid gas transactions to sequencer and receives a validation error
		if err := spammer.InvalidGasTx(config, airdropValue); err != nil {
			fmt.Printf("Error sending invalid gas transactions: %v\n", err)
			return err
		}
	}
	if config.InvalidNonce {
		if err := spammer.InvalidNonceTx(config, airdropValue); err != nil {
			fmt.Printf("Error sending invalid nonce transactions: %v\n", err)
			return err
		}
	}
	if config.InvalidNegativeValue {
		// generates a local problem, the marshalBinary function does not encode negative values
		if err := spammer.InvalidNegativeValueTx(config, airdropValue); err != nil {
			fmt.Printf("Error sending invalid negative value transactions: %v\n", err)
			return err
		}
	}
	if config.InvalidGasPriceZero {
		// works fine, sends invalid gas price zero transactions to sequencer and receives a validation error (transaction underpriced)
		if err := spammer.InvalidGasPriceZeroTx(config, airdropValue); err != nil {
			fmt.Printf("Error sending invalid gas price zero transactions: %v\n", err)
			return err
		}
	}
	if config.InvalidSignature {
		// works fine, sends invalid signature transactions to sequencer and receives a validation error (invalid signature)
		if err := spammer.InvalidSignatureTx(config, airdropValue); err != nil {
			fmt.Printf("Error sending invalid signature transactions: %v\n", err)
			return err
		}
	}
	// works fine, sends invalid chain id to sequencer and receives a validation error (invalid chain id)
	if config.InvalidChainId {
		if err := spammer.InvalidChainIdTx(config, airdropValue); err != nil {
			fmt.Printf("Error sending invalid chain ID transactions: %v\n", err)
			return err
		}
	}
	// works fine, sends txs from an unfunded address to sequencer and receives a validation error (insufficient funds for gas * price + value)
	// usually already in the gas estimation the error is returned, but here we force a fixedGasLimit
	if config.LackOfFundsTx {
		if err := spammer.LackOfFundsTx(config, airdropValue); err != nil {
			fmt.Printf("Error sending lack of fund transactions: %v\n", err)
			return err
		}
	}

	time.Sleep(time.Duration(config.SlotTime) * time.Second)
	return nil
}

func depositValidTx(config *spammer.Config) error {
	fmt.Printf("depositValidTx\n")
	if err := spammer.DepositValidTx(config); err != nil {
		fmt.Printf("Error: %v\n", err)
		return err
	}
	time.Sleep(time.Duration(config.SlotTime) * time.Second)
	return nil
}

// func singleBlob(config *spammer.Config, airdropValue *big.Int) error {
// 	fmt.Printf("singleBlob\n")
// 	if err := spammer.BlobTx(config, airdropValue); err != nil {
// 		fmt.Printf("Error: %v\n", err)
// 		return err
// 	}
// 	time.Sleep(time.Duration(config.SlotTime) * time.Second)
// 	return nil
// }

func runBasicSpam(c *cli.Context) error {
	config, err := spammer.NewConfigFromContext(c)
	if err != nil {
		return err
	}
	airdropValue := new(big.Int).Mul(big.NewInt(int64((1+config.N)*1000000)), big.NewInt(params.GWei))
	return spam(config, spammer.SendBasicTransactions, airdropValue)
}

func runSingleSpam(c *cli.Context) error {
	config, err := spammer.NewConfigFromContext(c)
	if err != nil {
		return err
	}
	airdropValue := new(big.Int).Mul(big.NewInt(int64((1+config.N)*1000000)), big.NewInt(params.GWei))
	return singleSpam(config, airdropValue)
}

// func runSingleBlob(c *cli.Context) error {
// 	fmt.Printf("runSingleBlob\n")
// 	config, err := spammer.NewConfigFromContext(c)
// 	if err != nil {
// 		return err
// 	}
// 	airdropValue := new(big.Int).Mul(big.NewInt(int64((1+config.N)*1000000)), big.NewInt(params.GWei))
// 	airdropValue = airdropValue.Mul(airdropValue, big.NewInt(100))
// 	return singleBlob(config, airdropValue)
// }

// func runBlobSpam(c *cli.Context) error {
// 	config, err := spammer.NewConfigFromContext(c)
// 	if err != nil {
// 		return err
// 	}
// 	airdropValue := new(big.Int).Mul(big.NewInt(int64((1+config.N)*1000000)), big.NewInt(params.GWei))
// 	airdropValue = airdropValue.Mul(airdropValue, big.NewInt(100))
// 	return spam(config, spammer.SendBlobTransactions, airdropValue)
// }

func runCreate(c *cli.Context) error {
	spammer.CreateAddresses(100)
	return nil
}

func runUnstuck(c *cli.Context) error {
	config, err := spammer.NewConfigFromContext(c)
	if err != nil {
		return err
	}
	return spammer.Unstuck(config)
}

func runDepositValidTx(c *cli.Context) error {
	config, err := spammer.NewConfigFromContext(c)
	if err != nil {
		return err
	}
	return depositValidTx(config)
}
