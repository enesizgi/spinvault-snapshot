const { ethers } = require("ethers");
const dotenv = require('dotenv');
const fs = require('fs');
dotenv.config();

function parseSwapEventData(data) {
  return data.result.map((log) => {
    const amount0In = log.data.slice(2,66);
    const amount1In = log.data.slice(66,130);
    const amount0Out = log.data.slice(130,194);
    const amount1Out = log.data.slice(194,258);
    return {
      ...log,
      data: {
        amount0In: BigInt('0x' + amount0In).toString(),
        amount1In: BigInt('0x' + amount1In).toString(),
        amount0Out: BigInt('0x' + amount0Out).toString(),
        amount1Out: BigInt('0x' + amount1Out).toString(),
      }
    };
  });
}

async function main() {
  const stakeEventHash = "0x9e71bc8eea02a63969f509818f2dafb9254532904319f9dbda79b67bd34a5f3d";
  const vaultDeployBlockNumber = 16654484;
  const spinStakableAddress = "0x06f2ba50843e2d26d8fd3184eaadad404b0f1a67";
  const spinStarterVaultAddress = "0x03447d28FC19cD3f3cB449AfFE6B3725b3BCdA77";
  const transferEventHash = "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef";
  const bscMainnetProvider = new ethers.JsonRpcProvider(process.env.RPC_URL);
  const currentBlockNumber = await bscMainnetProvider.getBlockNumber();

  let lastFetchedBlockNumber = vaultDeployBlockNumber - 1;
  const holders = new Set();
  let lastTurnCounter = 0;
  while (true) {
    const params = new URLSearchParams();
    params.append('module', 'logs');
    params.append('action', 'getLogs');
    params.append('fromBlock', lastFetchedBlockNumber.toString());
    params.append('toBlock', currentBlockNumber.toString());
    params.append('address', spinStarterVaultAddress);
    params.append('topic0', transferEventHash);
    params.append('topic1', '0x0000000000000000000000000000000000000000000000000000000000000000');
    params.append('topic0_1_opr', 'and');
    params.append('apikey', process.env.BSCSCAN_API_KEY);
    const tokenBuyLogs = await fetch(`https://api.bscscan.com/api?${params.toString()}`, {
      method: 'GET',
    });
    const data = await tokenBuyLogs.json();
    console.log(data);
    const data_holders = data.result.map((log) => {
      return log.topics[2].replace('0x000000000000000000000000', '0x');
    });
    data_holders.forEach((holder) => {
      holders.add(holder);
    });
    const maxBlockNumber = data.result.reduce((acc, log) => {
      return Math.max(acc, parseInt(log.blockNumber));
    }, 0);
    if (lastFetchedBlockNumber === maxBlockNumber - 1) {
      lastTurnCounter++;
    }
    lastFetchedBlockNumber = maxBlockNumber - 1;
    if (lastTurnCounter >= 2) {
      break;
    }
    console.log(lastFetchedBlockNumber);
  }
  console.log(holders);
  const balances = {};
  for await (const holder of holders) {
    const erc20Abi = [
      "function balanceOf(address owner) view returns (uint256)",
    ];
    const erc20Contract = new ethers.Contract(spinStarterVaultAddress, erc20Abi, bscMainnetProvider);
    const balance = await erc20Contract.balanceOf(holder);
    balances[holder] = balance.toString();
    if (balance !== '0') {
      console.log(holder, balance.toString());
    }
  }
  console.log(holders);
  // fs.writeFileSync('output.csv', csvData);
}

main().then(() => {}).catch((err) => {
  console.log(err);
  process.exitCode = 1
})
