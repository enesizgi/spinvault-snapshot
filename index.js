const { ethers } = require("ethers");
const dotenv = require('dotenv');
const fs = require('fs');
dotenv.config();

// Node.js
const { Client } = require('@bnb-chain/greenfield-js-sdk');
// const { ACCOUNT_ADDRESS, ACCOUNT_PRIVATEKEY } = require('./env');

async function main() {
  const vaultDeployBlockNumber = 16654484;
  const spinStakableAddress = "0x06f2ba50843e2d26d8fd3184eaadad404b0f1a67";
  const spinStarterVaultAddress = "0x03447d28FC19cD3f3cB449AfFE6B3725b3BCdA77";
  const spinTokenAddress = "0x6AA217312960A21aDbde1478DC8cBCf828110A67";
  const transferEventHash = "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef";
  const bscMainnetProvider = new ethers.JsonRpcProvider(process.env.RPC_URL);
  const currentBlockNumber = await bscMainnetProvider.getBlockNumber();
  const now = new Date();

  let lastFetchedBlockNumber = vaultDeployBlockNumber - 1;
  let holders = new Set();
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
  const stakedSpinBalances = {};

  // TODO: Remove this. Only for test.
  // holders = Array.from(holders).slice(0, 20);

  const erc20Abi = [
    "function balanceOf(address owner) view returns (uint256)",
    "function getUserStaked(address account) view returns (uint256)",
    "function totalSupply() view returns (uint256)"
  ];
  const spinStarterVaultContract = new ethers.Contract(spinStarterVaultAddress, erc20Abi, bscMainnetProvider);
  const totalSupply = await spinStarterVaultContract.totalSupply();

  const spinTokenContract = new ethers.Contract(spinTokenAddress, erc20Abi, bscMainnetProvider);
  const spinTokenTotalSupply = await spinTokenContract.totalSupply();

  for await (const holder of holders) {
    const [balance, stakedBalance] = await Promise.all([
      spinStarterVaultContract.balanceOf(holder),
      spinStarterVaultContract.getUserStaked(holder),
    ]);
    balances[holder] = balance.toString();
    stakedSpinBalances[holder] = stakedBalance.toString();
    console.log(holder, balance.toString());
  }

  const totalSpinStaked = Object.values(stakedSpinBalances).reduce((acc, balance) => {
    return acc + BigInt(balance.toString());
  }, BigInt(0));

  const fileJson = {
    isoString: now.toISOString(),
    fromBlock: vaultDeployBlockNumber,
    toBlock: currentBlockNumber,
    totalVaultShares: totalSupply.toString(),
    vaultShares: balances,
    spinTokenTotalSupply: spinTokenTotalSupply.toString(),
    totalSpinStaked: totalSpinStaked.toString(),
    stakedSpinBalances,
  };
  fs.writeFileSync(`snapshot-${now.toISOString()}.json`, JSON.stringify(fileJson));
}

main().then(() => {}).catch((err) => {
  console.log(err);
  process.exitCode = 1
})
