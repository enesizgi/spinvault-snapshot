package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"strings"
	"time"

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

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("err loading: %v", err)
	}
	fmt.Println("Starting snapshot app...")

	onlyUpload := flag.Bool("onlyUpload", false, "only upload last snapshot to greenfield without sweeping holders")
	onlySnapshot := flag.Bool("onlySnapshot", false, "only snapshot without uploading to greenfield")
	dev := flag.Bool("dev", false, "use dev environment")
	flag.Parse()

	for {
		// get all files in current directory
		files, err := ioutil.ReadDir(".")
		if err != nil {
			log.Fatal(err)
		}

		// get latest snapshot date
		var latestSnapshotDate time.Time
		var latestSnapshotFileName string
		for _, file := range files {
			if strings.HasPrefix(file.Name(), "snapshot-") && strings.HasSuffix(file.Name(), ".json") {
				latestSnapshotFileName = file.Name()
				snapshotDate, err := time.Parse("snapshot-2006-01-02T15:04:05Z.json", file.Name())
				if err != nil {
					log.Fatal(err)
				}
				if snapshotDate.After(latestSnapshotDate) {
					latestSnapshotDate = snapshotDate
				}
			}
		}

		if onlySnapshot != nil && *onlySnapshot && time.Since(latestSnapshotDate) > 24*time.Hour {
			_, err = sweepHolders()
			if err != nil {
				fmt.Println("Error sweeping holders:", err)
			}
		} else if onlyUpload != nil && *onlyUpload {
			uploadToGreenfield(true, latestSnapshotFileName)
			return
		} else if time.Since(latestSnapshotDate) > 24*time.Hour {
			uploadToGreenfield(false, "")
		} else if dev != nil && *dev {
			_, err = sweepHolders()
			if err != nil {
				fmt.Println("Error sweeping holders:", err)
			}
		}
		time.Sleep(10 * time.Second)
	}
}
