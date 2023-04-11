package main

import (
	"bufio"
	"context"
	"fmt"
	"github.com/amazechain/amc/params"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"math/big"
	"os"
	"strings"
	"time"
)

func main() {

	client, err := ethclient.Dial("ws://18.134.94.52:20013")
	if err != nil {
		panic("cannot connect to node")
	}

	//fmt.Println("Please enter the file name.")
	//var filename string
	//fmt.Scan(&filename)
	f, e := os.Open("/Users/mac/work/metachain/cmd/balance/database.csv")
	if e != nil {
		fmt.Println("File error.")
	} else {
		ctx, _ := context.WithTimeout(context.Background(), 10000*time.Second)
		buf := bufio.NewScanner(f)
		balanceMap := make(map[common.Address]*big.Int)
		for {
			if !buf.Scan() {
				break
			}

			addrString := buf.Text()
			addrString = strings.TrimSpace(addrString)
			//
			account := common.HexToAddress(addrString)

			if _, ok := balanceMap[account]; ok {
				continue
			}

			balance, err := client.BalanceAt(ctx, account, nil)
			//
			if err != nil {
				panic(fmt.Sprintf("Cannot fetch balance err: %v", err))
			}
			nonce, err := client.NonceAt(ctx, account, nil)
			//
			if err != nil {
				panic(fmt.Sprintf("Cannot fetch nonce err: %v", err))
			}

			balanceMap[account] = balance

			// allocs
			//addrStrings := strings.Split(addrString, "0x")
			//fmt.Printf("{\"address\": \"AMC%s\", \"balance\": \"%s\"},\n", addrStrings[1], balance.String())

			fmt.Printf("%s,%d,%.2f\n",
				addrString,
				nonce,
				new(big.Float).Quo(new(big.Float).SetInt(balance), new(big.Float).SetInt(big.NewInt(params.AMT))),
			)
		}

	}

}
