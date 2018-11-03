# UTXO-splitter

**Are you about to send hundreds or even thousands of transactions to the NEO network?**  
If so, be a good citizen by attaching a gas fee to each transaction.

This script will help split an UTXO into `N TXs` of `X gas` so you can use each TX as a transaction input for a gas fee.


### Usage
1. close this project
2. Install neo-utils by `go get github.com/o3labs/neo-utils`
3. open `main.go`
4. edit `numberOfSplits` and `amountOfGas` and `network` variables to suits your need. 
5. run it by `go run main.go`
6. your will see the rax transaction hex string that is ready to be sent with `sendrawtransaction` 


### Example result
https://neoscan-testnet.io/transaction/05FA9AE3302FEC755292266A7115E9F81189A033D5B95DD34D09F82E0259ED0
