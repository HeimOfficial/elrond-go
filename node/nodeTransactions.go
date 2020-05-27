package node

import (
	"encoding/hex"
	"fmt"

	rewardTxData "github.com/ElrondNetwork/elrond-go/data/rewardTx"
	"github.com/ElrondNetwork/elrond-go/data/smartContractResult"
	"github.com/ElrondNetwork/elrond-go/data/transaction"
	"github.com/ElrondNetwork/elrond-go/dataRetriever"
)

type transactionType string

const (
	moveBalanceTx transactionType = "moveBalance"
	unsignedTx    transactionType = "unsignedTx"
	rewardTx      transactionType = "rewardTx"
	invalidTx     transactionType = "invalidTx"
)

// GetTransaction gets the transaction
func (n *Node) GetTransaction(txHash string) (*transaction.ApiTransactionResult, error) {
	if !n.apiTransactionByHashThrottler.CanProcess() {
		return nil, ErrSystemBusyTxHash
	}

	n.apiTransactionByHashThrottler.StartProcessing()
	defer n.apiTransactionByHashThrottler.EndProcessing()

	hash, err := hex.DecodeString(txHash)
	if err != nil {
		return nil, err
	}

	txBytes, txType, found := n.getTxFromDataPool(hash)
	if found {
		return n.convertBytesToTransaction(txBytes, txType)
	}

	txBytes, txType, found = n.getTxFromStorage(hash)
	if found {
		return n.convertBytesToTransaction(txBytes, txType)
	}

	return nil, fmt.Errorf("transaction not found")
}

// GetTransactionStatus gets the transaction status
func (n *Node) GetTransactionStatus(txHash string) (string, error) {
	if !n.apiTransactionByHashThrottler.CanProcess() {
		return "", ErrSystemBusyTxHash
	}

	n.apiTransactionByHashThrottler.StartProcessing()
	defer n.apiTransactionByHashThrottler.EndProcessing()

	hash, err := hex.DecodeString(txHash)
	if err != nil {
		return "", err
	}

	_, _, foundInDataPool := n.getTxFromDataPool(hash)
	if foundInDataPool {
		return "received", nil
	}

	foundInStorage := n.isTxInStorage(hash)
	if foundInStorage {
		return "executed", nil
	}

	return "unknown", nil
}

func (n *Node) getTxFromDataPool(hash []byte) ([]byte, transactionType, bool) {
	txsPool := n.dataPool.Transactions()
	txBytes, found := txsPool.SearchFirstData(hash)
	if found && txBytes != nil {
		return txBytes.([]byte), moveBalanceTx, true
	}

	rewardTxsPool := n.dataPool.RewardTransactions()
	txBytes, found = rewardTxsPool.SearchFirstData(hash)
	if found && txBytes != nil {
		return txBytes.([]byte), rewardTx, true
	}

	unsignedTxsPool := n.dataPool.UnsignedTransactions()
	txBytes, found = unsignedTxsPool.SearchFirstData(hash)
	if found && txBytes != nil {
		return txBytes.([]byte), unsignedTx, true
	}

	return nil, invalidTx, false
}

func (n *Node) isTxInStorage(hash []byte) bool {
	txsStorer := n.store.GetStorer(dataRetriever.TransactionUnit)
	err := txsStorer.Has(hash)
	if err == nil {
		return true
	}

	rewardTxsStorer := n.store.GetStorer(dataRetriever.RewardTransactionUnit)
	err = rewardTxsStorer.Has(hash)
	if err == nil {
		return true
	}

	unsignedTransactionsStorer := n.store.GetStorer(dataRetriever.UnsignedTransactionUnit)
	err = unsignedTransactionsStorer.Has(hash)
	return err == nil
}

func (n *Node) getTxFromStorage(hash []byte) ([]byte, transactionType, bool) {
	txsStorer := n.store.GetStorer(dataRetriever.TransactionUnit)
	txBytes, err := txsStorer.SearchFirst(hash)
	if err == nil {
		return txBytes, moveBalanceTx, true
	}

	rewardTxsStorer := n.store.GetStorer(dataRetriever.RewardTransactionUnit)
	txBytes, err = rewardTxsStorer.SearchFirst(hash)
	if err == nil {
		return txBytes, rewardTx, true
	}

	unsignedTransactionsStorer := n.store.GetStorer(dataRetriever.UnsignedTransactionUnit)
	txBytes, err = unsignedTransactionsStorer.SearchFirst(hash)
	if err == nil {
		return txBytes, unsignedTx, true
	}

	return nil, invalidTx, false
}

func (n *Node) convertBytesToTransaction(txBytes []byte, txType transactionType) (*transaction.ApiTransactionResult, error) {
	switch txType {
	case moveBalanceTx:
		var tx transaction.Transaction
		err := n.internalMarshalizer.Unmarshal(&tx, txBytes)
		if err != nil {
			return nil, err
		}
		return &transaction.ApiTransactionResult{
			Type:      string(moveBalanceTx),
			Nonce:     tx.Nonce,
			Value:     tx.Value.String(),
			Receiver:  n.addressPubkeyConverter.Encode(tx.RcvAddr),
			Sender:    n.addressPubkeyConverter.Encode(tx.SndAddr),
			GasPrice:  tx.GasPrice,
			GasLimit:  tx.GasLimit,
			Data:      string(tx.Data),
			Signature: hex.EncodeToString(tx.Signature),
		}, nil
	case rewardTx:
		var tx rewardTxData.RewardTx
		err := n.internalMarshalizer.Unmarshal(&tx, txBytes)
		if err != nil {
			return nil, err
		}
		return &transaction.ApiTransactionResult{
			Type:      string(rewardTx),
			Nonce:     tx.GetNonce(),
			Value:     tx.GetValue().String(),
			Receiver:  n.addressPubkeyConverter.Encode(tx.GetRcvAddr()),
			Sender:    n.addressPubkeyConverter.Encode(tx.GetSndAddr()),
			GasPrice:  tx.GetGasPrice(),
			GasLimit:  tx.GetGasLimit(),
			Data:      string(tx.GetData()),
			Signature: hex.EncodeToString(tx.GetData()),
		}, nil

	case unsignedTx:
		var tx smartContractResult.SmartContractResult
		err := n.internalMarshalizer.Unmarshal(&tx, txBytes)
		if err != nil {
			return nil, err
		}
		return &transaction.ApiTransactionResult{
			Type:      string(unsignedTx),
			Nonce:     tx.GetNonce(),
			Value:     tx.GetValue().String(),
			Receiver:  n.addressPubkeyConverter.Encode(tx.GetRcvAddr()),
			Sender:    n.addressPubkeyConverter.Encode(tx.GetSndAddr()),
			GasPrice:  tx.GetGasPrice(),
			GasLimit:  tx.GetGasLimit(),
			Data:      string(tx.GetData()),
			Signature: "",
		}, nil
	default:
		return &transaction.ApiTransactionResult{Type: string(invalidTx)}, nil // this shouldn't happen
	}
}
