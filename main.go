package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/o3labs/neo-utils/neoutils"
	"github.com/o3labs/neo-utils/neoutils/o3"
	"github.com/o3labs/neo-utils/neoutils/smartcontract"
)

func utxoFromO3Platform(network string, address string) (smartcontract.Unspent, error) {

	unspent := smartcontract.Unspent{
		Assets: map[smartcontract.NativeAsset]*smartcontract.Balance{},
	}

	client := o3.DefaultO3APIClient()
	if network == "test" {
		client = o3.APIClientWithNEOTestnet()
	}

	response := client.GetNEOUTXO(address)
	if response.Code != 200 {
		return unspent, fmt.Errorf("Error cannot get utxo")
	}

	gasBalance := smartcontract.Balance{
		Amount: float64(0),
		UTXOs:  []smartcontract.UTXO{},
	}

	neoBalance := smartcontract.Balance{
		Amount: float64(0),
		UTXOs:  []smartcontract.UTXO{},
	}

	for _, v := range response.Result.Data {
		if strings.Contains(v.Asset, string(smartcontract.GAS)) {
			value, err := strconv.ParseFloat(v.Value, 64)
			if err != nil {
				continue
			}
			gasTX1 := smartcontract.UTXO{
				Index: v.Index,
				TXID:  v.Txid,
				Value: value,
			}
			gasBalance.UTXOs = append(gasBalance.UTXOs, gasTX1)
		}

		if strings.Contains(v.Asset, string(smartcontract.NEO)) {
			value, err := strconv.ParseFloat(v.Value, 64)
			if err != nil {
				continue
			}
			tx := smartcontract.UTXO{
				Index: v.Index,
				TXID:  v.Txid,
				Value: value,
			}
			neoBalance.UTXOs = append(neoBalance.UTXOs, tx)
		}
	}

	unspent.Assets[smartcontract.GAS] = &gasBalance
	unspent.Assets[smartcontract.NEO] = &neoBalance
	return unspent, nil
}

func buildTXOutputs(unspent smartcontract.Unspent, toAddress string, asset smartcontract.NativeAsset, numberOfOutput uint, amount float64) ([]byte, error) {

	sendingAsset := unspent.Assets[asset]

	if amount > sendingAsset.TotalAmount() {
		return nil, fmt.Errorf("you don't have enough balance. Sending %v but only have %v", amount, sendingAsset.TotalAmount())
	}
	//sort min first
	sendingAsset.SortMinFirst()

	utxoSumAmount := float64(0)
	index := 0

	amountOfGasNeedForSplit := float64(float64(numberOfOutput) * amount)

	//figure out whehter we need a change as another ouput
	for utxoSumAmount < amountOfGasNeedForSplit {
		addingUTXO := sendingAsset.UTXOs[index]
		utxoSumAmount += addingUTXO.Value
		index += 1
	}
	totalAmountInInputs := utxoSumAmount
	needChange := totalAmountInInputs > amountOfGasNeedForSplit

	outputBuilder := smartcontract.NewScriptBuilder()

	list := []smartcontract.TransactionOutput{}
	for i := 0; i < int(numberOfOutput); i++ {
		output := smartcontract.TransactionOutput{
			Asset:   asset,
			Value:   int64(smartcontract.RoundFixed8(amount) * float64(100000000)),
			Address: smartcontract.ParseNEOAddress(toAddress),
		}
		list = append(list, output)
	}

	if needChange == true {
		returningAmount := totalAmountInInputs - amountOfGasNeedForSplit
		returningOutput := smartcontract.TransactionOutput{
			Asset:   asset,
			Value:   int64(smartcontract.RoundFixed8(returningAmount) * float64(100000000)),
			Address: smartcontract.ParseNEOAddress(toAddress),
		}
		list = append(list, returningOutput)
	}

	outputBuilder.PushLength(len(list))
	for _, v := range list {
		outputBuilder.Push(v)
	}

	return outputBuilder.ToBytes(), nil
}

func main() {
	//uncomment this part if you want to use the nep2 encrypted key

	// encryptedKey := ""
	// password := ""
	// wif, err := neoutils.NEP2Decrypt(encryptedKey, password)
	// if err != nil {
	// 	fmt.Printf("Invalid password or encrypted key")
	// 	return
	// }
	// end using encrypted key

	wif := ""
	wallet, err := neoutils.GenerateFromWIF(wif)
	if err != nil {
		fmt.Printf("Invalid wif")
		return
	}

	assetToSplit := smartcontract.GAS

	numberOfSplits := uint(100)     //number of tx needed
	amountOfGas := float64(0.00001) //amount of gas in each utxo
	network := "test"

	amountOfGasNeedForSplit := float64(float64(numberOfSplits) * amountOfGas)

	tx := smartcontract.NewContractTransaction()

	unspent, err := utxoFromO3Platform(network, wallet.Address)
	if err != nil {
		fmt.Printf("%v", err)
		return
	}
	filteredUnspent := smartcontract.Unspent{
		Assets: map[smartcontract.NativeAsset]*smartcontract.Balance{},
	}

	b := smartcontract.Balance{
		Amount: float64(0) / float64(100000000),
		UTXOs:  []smartcontract.UTXO{},
	}

	//because we are splitting utxo
	//it's best to use the utxo tx that has value more than total sum of split
	inputList := []smartcontract.UTXO{}
	for _, u := range unspent.Assets[assetToSplit].UTXOs {
		if u.Value > amountOfGasNeedForSplit {
			inputList = append(inputList, u)
			tx := smartcontract.UTXO{
				Index: u.Index,
				TXID:  u.TXID,
				Value: u.Value,
			}
			b.UTXOs = append(b.UTXOs, tx)
			continue
		}
	}

	if len(inputList) == 0 {
		fmt.Printf("unable to find input to split")
		return
	}

	filteredUnspent.Assets[assetToSplit] = &b

	inputBuilder := smartcontract.NewScriptBuilder()
	inputBuilder.PushLength(len(inputList))
	for _, v := range inputList {
		inputBuilder.Push(v)
	}

	tx.Inputs = inputBuilder.ToBytes()

	outputs, err := buildTXOutputs(filteredUnspent, wallet.Address, assetToSplit, numberOfSplits, amountOfGas)
	if err != nil {
		fmt.Printf("%v", err)
		return
	}
	tx.Outputs = outputs

	attributes := map[smartcontract.TransactionAttribute][]byte{}
	attributes[smartcontract.Remark] = []byte("O3XGASSPLIITER")
	txAttributes, err := smartcontract.NewScriptBuilder().GenerateTransactionAttributes(attributes)
	if err != nil {
		fmt.Printf("%v", err)
		return
	}

	tx.Attributes = txAttributes

	//begin signing
	privateKeyInHex := neoutils.BytesToHex(wallet.PrivateKey)
	signedData, err := neoutils.Sign(tx.ToBytes(), privateKeyInHex)
	if err != nil {
		fmt.Printf("err signing %v", err)
		return
	}

	signature := smartcontract.TransactionSignature{
		SignedData: signedData,
		PublicKey:  wallet.PublicKey,
	}

	scripts := []interface{}{signature}
	verificationScripts := smartcontract.NewScriptBuilder().GenerateVerificationScripts(scripts)

	endPayload := []byte{}
	endPayload = append(endPayload, tx.ToBytes()...)
	endPayload = append(endPayload, verificationScripts...)

	log.Printf("txID: %v\n", tx.ToTXID())
	log.Printf("raw tx to send: %x\n", endPayload)
}
