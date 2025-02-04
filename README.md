# TX-Fuzz

TX-Fuzz is a package containing helpful functions to create random transactions. 
It can be used to easily access fuzzed transactions from within other programs.

## Usage

```
cd cmd/livefuzzer
go build
```

Run an execution layer client such as [Geth][1] locally in a standalone bash window.
Tx-fuzz sends transactions to port `8545` by default.

```
geth --http --http.port 8545
```

Run livefuzzer.

```
./livefuzzer spam
```
if the above command fails, try to run it in debug mode:

```
lldb ./livefuzzer
run spam
```

To run the special flags to generate invalid transactions, use the `singleSpam` command that will send 100 transactions, and optionally add these flags
`--invalid-gas`
`--invalid-nonce`
`--invalid-negative-value`
`--invalid-gas-price-zero`
`--invalid-signature`
`--invalid-chain-id`
`--lack-of-funds`
Instead to run the blob spammer, use the `singleBlob` command. This will generate 100 random blob transactions.

example:
```
./livefuzzer singleSpam --invalid-gas --invalid-nonce
```


Tx-fuzz allows for an optional seed parameter to get reproducible fuzz transactions

## Advanced usage
You can optionally specify a seed parameter or a secret key to use as a faucet

```
./livefuzzer spam --seed <seed> --sk <SK>
```

You can set the RPC to use with `--rpc <RPC>`.
