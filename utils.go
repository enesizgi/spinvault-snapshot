package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/bnb-chain/greenfield-go-sdk/client"
	"github.com/bnb-chain/greenfield-go-sdk/types"
	types2 "github.com/bnb-chain/greenfield/x/storage/types"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"

	"io/ioutil"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

func sweepHolders() (filename string, err error) {
	rpcURL := os.Getenv("RPC_URL")
	rpcClient, err := ethclient.Dial(rpcURL)
	if err != nil {
		fmt.Println("Failed to connect to the Ethereum rpcClient:", err)
		return "", err
	}

	currentBlockNumber, err := rpcClient.BlockNumber(context.Background())
	if err != nil {
		fmt.Println("Failed to get current block number:", err)
		return "", err
	}

	now := time.Now()
	lastFetchedBlockNumber := vaultDeployBlockNumber - 1
	holders := make(map[string]bool)
	lastTurnCounter := 0

	for {
		values := url.Values{}
		values.Add("module", "logs")
		values.Add("action", "getLogs")
		values.Add("fromBlock", strconv.Itoa(lastFetchedBlockNumber))
		values.Add("toBlock", strconv.Itoa(int(currentBlockNumber)))
		values.Add("address", spinStarterVaultAddressStr)
		values.Add("topic0", transferEventHash)
		values.Add("topic1", "0x0000000000000000000000000000000000000000000000000000000000000000")
		values.Add("topic0_1_opr", "and")
		values.Add("apikey", os.Getenv("BSCSCAN_API_KEY"))

		resp, err := http.Get("https://api.bscscan.com/api?" + values.Encode())
		if err != nil {
			fmt.Println("Error fetching data:", err)
			return "", err
		}
		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			fmt.Println("Error reading response body:", err)
			return "", err
		}

		var bscResp BscScanResponse
		if err := json.Unmarshal(body, &bscResp); err != nil {
			fmt.Println("Error unmarshalling response:", err)
			return "", err
		}

		for _, logEvent := range bscResp.Result {
			holder := "0x" + logEvent.Topics[2][26:]
			holders[holder] = true
		}

		// more code follows...

		maxBlockNumber := 0
		for _, logEvent := range bscResp.Result {
			blockNum, err := strconv.ParseInt(logEvent.BlockNumber[2:], 16, 64)
			if err != nil || blockNum == 0 {
				fmt.Println("Error parsing block number:", err)
				return "", err
			}
			if blockNum > int64(maxBlockNumber) {
				maxBlockNumber = int(blockNum)
			}
		}
		if lastFetchedBlockNumber == maxBlockNumber-1 {
			lastTurnCounter++
		}
		lastFetchedBlockNumber = maxBlockNumber - 1
		if lastTurnCounter >= 2 {
			break
		}
	}

	//Convert holders map to a slice
	holderAddresses := make([]string, 0, len(holders))
	for addr := range holders {
		holderAddresses = append(holderAddresses, addr)
	}

	// Define the ERC20 ABI for balance and staking queries
	const erc20Abi = `[
		{
			"constant": true,
			"inputs": [{"name": "owner", "type": "address"}],
			"name": "balanceOf",
			"outputs": [{"name": "", "type": "uint256"}],
			"payable": false,
			"stateMutability": "view",
			"type": "function"
		},
		{
			"constant": true,
			"inputs": [{"name": "account", "type": "address"}],
			"name": "getUserStaked",
			"outputs": [{"name": "", "type": "uint256"}],
			"payable": false,
			"stateMutability": "view",
			"type": "function"
		},
		{
			"constant": true,
			"inputs": [],
			"name": "totalSupply",
			"outputs": [{"name": "", "type": "uint256"}],
			"payable": false,
			"stateMutability": "view",
			"type": "function"
		}
	]`

	// Parse the contract ABI
	parsedABI, err := abi.JSON(strings.NewReader(erc20Abi))
	if err != nil {
		panic(err)
	}

	// Pack the data to send (the address you want the balance of)
	spinStarterVaultAddress := common.HexToAddress(spinStarterVaultAddressStr) // Replace with the address you're querying
	data, err := parsedABI.Pack("totalSupply")
	if err != nil {
		panic(err)
	}

	msg := ethereum.CallMsg{
		To:   &spinStarterVaultAddress,
		Data: data,
	}
	totalSupplyResult, err := rpcClient.CallContract(context.Background(), msg, nil)
	if err != nil {
		fmt.Println("Error getting total supply:", err)
		panic(err)
	}
	totalSupplyUnpacked, err := parsedABI.Unpack("totalSupply", totalSupplyResult)
	if err != nil {
		fmt.Println("Error unpacking total supply:", err)
		panic(err)
	}
	totalSupply := totalSupplyUnpacked[0].(*big.Int)

	spinTokenAddress := common.HexToAddress(spinTokenAddress) // Replace with the address you're querying

	msg = ethereum.CallMsg{
		To:   &spinTokenAddress,
		Data: data,
	}
	spinTotalSupplyResult, err := rpcClient.CallContract(context.Background(), msg, nil)
	if err != nil {
		fmt.Println("Error getting total supply:", err)
		panic(err)
	}

	spinTotalSupplyUnpacked, err := parsedABI.Unpack("totalSupply", spinTotalSupplyResult)
	if err != nil {
		fmt.Println("Error unpacking total supply:", err)
		panic(err)
	}
	spinTotalSupply := spinTotalSupplyUnpacked[0].(*big.Int)

	balances := make(map[string]string)
	stakedSpinBalances := make(map[string]string)
	total_diff := 0 * time.Second
	fmt.Println(total_diff)
	total_cu := 0
	for _, holder := range holderAddresses {
		time1 := time.Now()
		data, err = parsedABI.Pack("balanceOf", common.HexToAddress(holder))
		if err != nil {
			fmt.Println("Error packing balanceOf:", err)
			panic(err)
		}
		msg = ethereum.CallMsg{
			To:   &spinStarterVaultAddress,
			Data: data,
		}
		balanceResult, err := rpcClient.CallContract(context.Background(), msg, nil)
		if err != nil {
			fmt.Println("Error getting balance:", err)
			continue
		}
		balanceUnpacked, err := parsedABI.Unpack("balanceOf", balanceResult)
		if err != nil {
			fmt.Println("Error unpacking balance:", err)
			panic(err)
		}
		balance := balanceUnpacked[0].(*big.Int)

		// get user Staked balance
		data, err = parsedABI.Pack("getUserStaked", common.HexToAddress(holder))
		if err != nil {
			fmt.Println("Error packing getUserStaked:", err)
			panic(err)
		}
		msg = ethereum.CallMsg{
			To:   &spinStarterVaultAddress,
			Data: data,
		}
		stakedBalanceResult, err := rpcClient.CallContract(context.Background(), msg, nil)
		if err != nil {
			fmt.Println("Error getting staked balance:", err)
			continue
		}
		stakedBalanceUnpacked, err := parsedABI.Unpack("getUserStaked", stakedBalanceResult)
		if err != nil {
			fmt.Println("Error unpacking staked balance:", err)
			panic(err)
		}
		stakedBalance := stakedBalanceUnpacked[0].(*big.Int)

		balances[holder] = balance.String()
		stakedSpinBalances[holder] = stakedBalance.String()
		fmt.Println(holder, balance, stakedBalance)
		time2 := time.Now()
		total_diff += time2.Sub(time1)
		total_cu += 25
		if total_cu >= 150 {
			total_cu = 0
			fmt.Println(total_diff, total_cu)
			if total_diff < 1*time.Second {
				time.Sleep(1*time.Second - total_diff)
			} else {
				time.Sleep(1 * time.Second)
			}
			total_diff = 0 * time.Second
		}
	}

	totalSpinStaked := big.NewInt(0)
	for _, balance := range stakedSpinBalances {
		bal, _ := new(big.Int).SetString(balance, 10)
		totalSpinStaked.Add(totalSpinStaked, bal)
	}

	snapshot := Snapshot{
		ISOString:            now.UTC().Format(time.RFC3339),
		FromBlock:            vaultDeployBlockNumber,
		ToBlock:              int(currentBlockNumber),
		TotalVaultShares:     totalSupply.String(),
		VaultShares:          balances,
		SpinTokenTotalSupply: spinTotalSupply.String(),
		TotalSpinStaked:      totalSpinStaked.String(),
		StakedSpinBalances:   stakedSpinBalances,
	}

	fileName := fmt.Sprintf("snapshot-%s.json", snapshot.ISOString)
	fileJson, _ := json.Marshal(snapshot)
	if err := ioutil.WriteFile(fileName, fileJson, 0644); err != nil {
		fmt.Println("Error writing file:", err)
	}
	return fileName, nil
}

func uploadToGreenfield(isOnlyUpload bool, snapshotFileName string) {
	var objectName string
	var err error
	if isOnlyUpload {
		fmt.Println("Uploading snapshot file:", snapshotFileName)
		objectName = snapshotFileName
	} else {
		fmt.Println("Sweeping holders...")
		objectName, err = sweepHolders()
	}
	if err != nil {
		fmt.Println("Error sweeping holders:", err)
		return
	}
	privateKey := os.Getenv("ACCOUNT_PRIVATEKEY")
	account, err := types.NewAccountFromPrivateKey("bnb", privateKey)
	if err != nil {
		log.Fatalf("New account from private key error, %v", err)
	}
	cli, err := client.New(chainId, rpcAddr, client.Option{DefaultAccount: account})
	if err != nil {
		log.Fatalf("unable to new greenfield client, %v", err)
	}
	ctx := context.Background()

	// head bucket
	bucketInfo, err := cli.HeadBucket(ctx, bucketName)
	//handleErr(err, "HeadBucket")
	log.Println("bucket info:", bucketInfo.String())

	// Create object content
	// Read file content of objectName to buffer
	file, err := os.Open(objectName)
	if err != nil {
		log.Fatalf("fail to open file %s", objectName)
	}
	defer file.Close()
	fileInfo, err := file.Stat()
	if err != nil {
		log.Fatalf("fail to get file info %s", objectName)
	}
	objectSize := fileInfo.Size()
	buffer := make([]byte, objectSize)
	_, err = file.Read(buffer)
	if err != nil {
		log.Fatalf("fail to read file %s", objectName)
	}

	// create and put object
	txnHash, err := cli.CreateObject(ctx, bucketName, objectName, bytes.NewReader(buffer), types.CreateObjectOptions{
		Visibility: types2.VISIBILITY_TYPE_PUBLIC_READ,
	})
	if err != nil {
		log.Fatalf("fail to create object %s", objectName)
	}
	fmt.Println("txnHash:", txnHash)
	err = cli.PutObject(ctx, bucketName, objectName, objectSize,
		bytes.NewReader(buffer), types.PutObjectOptions{TxnHash: txnHash})
	if err != nil {
		log.Fatalf("fail to put object %s", objectName)
	}
	log.Printf("object: %s has been uploaded to SP\n", objectName)

	waitObjectSeal(cli, bucketName, objectName)
	// wait for block_syncer to sync up data from chain
	time.Sleep(time.Second * 5)
}

func handleErr(err error, funcName string) {
	if err != nil {
		log.Fatalln("fail to " + funcName + ": " + err.Error())
	}
}

func waitObjectSeal(cli client.IClient, bucketName, objectName string) {
	ctx := context.Background()
	// wait for the object to be sealed
	timeout := time.After(15 * time.Second)
	ticker := time.NewTicker(2 * time.Second)

	for {
		select {
		case <-timeout:
			err := errors.New("object not sealed after 15 seconds")
			handleErr(err, "HeadObject")
		case <-ticker.C:
			objectDetail, err := cli.HeadObject(ctx, bucketName, objectName)
			handleErr(err, "HeadObject")
			if objectDetail.ObjectInfo.GetObjectStatus().String() == "OBJECT_STATUS_SEALED" {
				ticker.Stop()
				fmt.Printf("put object %s successfully \n", objectName)
				return
			}
		}
	}
}

func uploadMissingAfterLastUploaded(localSnapshots []string) {
	privateKey := os.Getenv("ACCOUNT_PRIVATEKEY")
	account, err := types.NewAccountFromPrivateKey("bnb", privateKey)
	if err != nil {
		log.Fatalf("New account from private key error, %v", err)
	}
	cli, err := client.New(chainId, rpcAddr, client.Option{DefaultAccount: account})
	if err != nil {
		log.Fatalf("unable to new greenfield client, %v", err)
	}
	ctx := context.Background()

	// head bucket
	bucketInfo, err := cli.HeadBucket(ctx, bucketName)
	//handleErr(err, "HeadBucket")
	log.Println("bucket info:", bucketInfo.String())

	// list objects
	objects, err := cli.ListObjects(ctx, bucketName, types.ListObjectsOptions{
		ShowRemovedObject: false, Delimiter: "", MaxKeys: 100,
		Endpoint:  "https://greenfield-sp.ninicoin.io",
		SPAddress: "0x2901FDdEF924f077Ec6811A4a6a1CB0F13858e8f",
	})
	log.Println("list objects result:")

	var objectNames []string
	for _, obj := range objects.Objects {
		objectNames = append(objectNames, obj.ObjectInfo.ObjectName)
		//i := obj.ObjectInfo.ObjectName
		//log.Printf("object: %s, status: %s\n", i.ObjectName, i.ObjectStatus)
	}

	for _, localSnapshot := range localSnapshots {
		// check if local snapshot is in objectNames
		if slices.Contains(objectNames, localSnapshot) {
			continue
		}

		fmt.Println("Uploading missing snapshot file:", localSnapshot)
		uploadToGreenfield(true, localSnapshot)
	}
}
