package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/bnb-chain/greenfield-go-sdk/client"
	"github.com/bnb-chain/greenfield-go-sdk/types"
	storageTypes "github.com/bnb-chain/greenfield/x/storage/types"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/joho/godotenv"
)

const (
	spinStarterVaultAddressStr = "0x03447d28FC19cD3f3cB449AfFE6B3725b3BCdA77"
	spinTokenAddress           = "0x6AA217312960A21aDbde1478DC8cBCf828110A67"
	transferEventHash          = "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"
	vaultDeployBlockNumber     = 16654484
)

const (
	bucketName = "spin"
	// Mainnet Info
	rpcAddr = "https://greenfield-chain.bnbchain.org:443"
	chainId = "greenfield_1017-1"

	// Testnet Info
	// rpcAddr     = "https://gnfd-testnet-fullnode-tendermint-us.bnbchain.org:443"
	// chainId     = "greenfield_5600-1"

)

type LogResult struct {
	BlockNumber string   `json:"blockNumber"`
	Topics      []string `json:"topics"`
}

type BscScanResponse struct {
	Status  string      `json:"status"`
	Message string      `json:"message"`
	Result  []LogResult `json:"result"`
}

type Snapshot struct {
	ISOString            string            `json:"isoString"`
	FromBlock            int               `json:"fromBlock"`
	ToBlock              int               `json:"toBlock"`
	TotalVaultShares     string            `json:"totalVaultShares"`
	VaultShares          map[string]string `json:"vaultShares"`
	SpinTokenTotalSupply string            `json:"spinTokenTotalSupply"`
	TotalSpinStaked      string            `json:"totalSpinStaked"`
	StakedSpinBalances   map[string]string `json:"stakedSpinBalances"`
}

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
	fmt.Println("holders:", holders, len(holders))

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
	fmt.Println("totalSupply:", totalSupply)

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
	fmt.Println("spinTotalSupply:", spinTotalSupply)

	balances := make(map[string]string)
	stakedSpinBalances := make(map[string]string)
	for _, holder := range holderAddresses {
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
		fmt.Println("balanceResult:", balance, holder)

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
		fmt.Println("stakedBalanceResult:", stakedBalance, holder)

		balances[holder] = balance.String()
		stakedSpinBalances[holder] = stakedBalance.String()
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
	fmt.Println("fileName:", fileName)
	return fileName, nil
}

// it is the example of basic storage SDKs usage
func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("err loading: %v", err)
	}
	objectName, err := sweepHolders()
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
		Visibility: storageTypes.VISIBILITY_TYPE_PUBLIC_READ,
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
}
