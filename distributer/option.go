package distributer

import (
	"bufio"
	"fmt"
	"io"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/anyswap/ANYToken-distribution/mongodb"
	"github.com/anyswap/ANYToken-distribution/params"
	"github.com/anyswap/ANYToken-distribution/tools"
	"github.com/fsn-dev/fsn-go-sdk/efsn/common"
)

const sampleCount = 4

var outputFile *os.File

// Option ditribute options
type Option struct {
	TotalValue  *big.Int
	StartHeight uint64 // start inclusive
	EndHeight   uint64 // end exclusive
	Exchange    string
	RewardToken string
	InputFile   string
	OutputFile  string
	DryRun      bool
}

func (opt *Option) deinit() {
	if outputFile != nil {
		outputFile.Close()
	}
}

func (opt *Option) checkAndInit() (err error) {
	if opt.TotalValue == nil || opt.TotalValue.Sign() <= 0 {
		return fmt.Errorf("wrong total value %v", opt.TotalValue)
	}
	if opt.StartHeight >= opt.EndHeight {
		return fmt.Errorf("empty range, start height %v >= end height %v", opt.StartHeight, opt.EndHeight)
	}
	if !params.IsConfigedExchange(opt.Exchange) {
		return fmt.Errorf("exchange %v is not configed", opt.Exchange)
	}
	latestBlock := capi.LoopGetLatestBlockHeader()
	if latestBlock.Number.Uint64() < opt.EndHeight {
		return fmt.Errorf("latest height %v is lower than end height %v", latestBlock.Number, opt.EndHeight)
	}
	if !common.IsHexAddress(opt.RewardToken) {
		return fmt.Errorf("wrong reward token: '%v'", opt.RewardToken)
	}
	err = opt.checkSenderRewardTokenBalance()
	if err != nil {
		return err
	}
	err = opt.openOutputFile()
	if err != nil {
		return err
	}
	return nil
}

func (opt *Option) openOutputFile() (err error) {
	if opt.OutputFile == "" {
		return nil
	}
	outputFile, err = os.OpenFile(opt.OutputFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	return err
}

func (opt *Option) checkSenderRewardTokenBalance() (err error) {
	sender := commonTxArgs.fromAddr
	rewardTokenAddr := common.HexToAddress(opt.RewardToken)
	var senderTokenBalance *big.Int
	for {
		senderTokenBalance, err = capi.GetTokenBalance(rewardTokenAddr, sender, nil)
		if err != nil {
			time.Sleep(time.Second)
			continue
		}
		if senderTokenBalance.Cmp(opt.TotalValue) < 0 {
			return fmt.Errorf("not enough reward token balance, %v < %v", senderTokenBalance, opt.TotalValue)
		}
		break
	}
	return nil
}

func (opt *Option) getAccounts() (accounts []common.Address, err error) {
	if opt.InputFile == "" {
		accounts = mongodb.FindAllAccounts(opt.Exchange)
		return accounts, nil
	}

	file, err := os.Open(opt.InputFile)
	if err != nil {
		return nil, fmt.Errorf("open %v failed. %v)", opt.InputFile, err)
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	for {
		lineData, _, errf := reader.ReadLine()
		if errf == io.EOF {
			break
		}
		line := strings.TrimSpace(string(lineData))
		if !common.IsHexAddress(line) {
			return nil, fmt.Errorf("found wrong address line %v", line)
		}
		account := common.HexToAddress(line)
		accounts = append(accounts, account)
	}

	return accounts, nil
}

func (opt *Option) getAccountsAndVolumes() (accounts []common.Address, volumes []*big.Int, err error) {
	if opt.InputFile == "" {
		accounts, volumes = mongodb.FindAccountVolumes(opt.Exchange, opt.StartHeight, opt.EndHeight)
		return accounts, volumes, nil
	}

	file, err := os.Open(opt.InputFile)
	if err != nil {
		return nil, nil, fmt.Errorf("open %v failed. %v)", opt.InputFile, err)
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	for {
		lineData, _, errf := reader.ReadLine()
		if errf == io.EOF {
			break
		}
		line := strings.TrimSpace(string(lineData))
		parts := strings.Split(line, " ")
		accountStr := parts[0]
		volumeStr := parts[1]
		if !common.IsHexAddress(accountStr) {
			return nil, nil, fmt.Errorf("wrong address in line %v", line)
		}
		volume, err := tools.GetBigIntFromString(volumeStr)
		if err != nil {
			return nil, nil, err
		}
		account := common.HexToAddress(line)
		accounts = append(accounts, account)
		volumes = append(volumes, volume)
	}

	return accounts, volumes, nil
}