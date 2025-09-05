package main

import (
	"encoding/json"
	"log"
	"github.com/btcsuite/btcd/chaincfg"
	"net/http"
	"bytes" 
	"io" 
	"fmt" 
	"time"
	"sync"
	// "crypto/ecdsa"
    // "crypto/elliptic"
    // "encoding/hex"
	// "github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/txscript"

) 
 
var (
	lastRequestTime    time.Time
	sharedAvgFee10     int64
	mu                 sync.Mutex
)

type FeeData struct {
	AvgFee10 int64 `json:"avgFee_90"`  //avgFee_50 avgFee_75
}

type Transaction struct {
	TxID    string `json:"txid"`
	Status  struct {
		Confirmed bool `json:"confirmed"`
	} `json:"status"`
}


type UTXO struct {
	TxID         string `json:"txid"`
	VOut         uint32    `json:"vout"`
	Value        int64 `json:"value"`
	Status       struct {
		Confirmed bool `json:"confirmed"`
	} `json:"status"`
}


func CheckTransactionConfirmation(txID string) (bool, error) {
	url := fmt.Sprintf("https://mempool.space/api/tx/%s", txID)

	resp, err := http.Get(url)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("failed to fetch transaction: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}

	var transaction Transaction
	if err := json.Unmarshal(body, &transaction); err != nil {
		return false, err
	}

	return transaction.Status.Confirmed, nil
}


func getUtxoByAddress(address string) (UTXO, error) {
	url := fmt.Sprintf("https://mempool.space/api/address/%s/utxo", address)
	resp, err := http.Get(url)
	if err != nil {
		return UTXO{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return UTXO{}, fmt.Errorf("failed to fetch UTXOs: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return UTXO{}, err
	}

	var utxos []UTXO
	if err := json.Unmarshal(body, &utxos); err != nil {
		return UTXO{}, err
	}

	// 找到最大 Amount 且 Confirmed 为 true 的 UTXO
	var maxUTXO UTXO
	for _, utxo := range utxos {
		if utxo.Status.Confirmed && utxo.Value > maxUTXO.Value {
			maxUTXO = utxo
		}
	}

	if maxUTXO.TxID == "" {
		return UTXO{}, fmt.Errorf("no confirmed UTXO found for address: %s", address)
	}

	return maxUTXO, nil
}

func fetchAvgFee() (int64, error) {
	mu.Lock()
	defer mu.Unlock()

	currentTime := time.Now()

	if currentTime.Sub(lastRequestTime) >= 60*time.Second {
		url := "https://mempool.space/api/v1/mining/blocks/fee-rates/100m"
		resp, err := http.Get(url)
		if err != nil {
			return 0, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return 0, fmt.Errorf("failed to fetch data: %s", resp.Status)
		}

		var data []FeeData
		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			return 0, err
		}

		// 获取最后一个元素
		if len(data) > 0 {
			lastItem := data[len(data)-1]

			// 更新共享变量
			if lastItem.AvgFee10 != 0 {
				lastRequestTime = currentTime
				sharedAvgFee10 = lastItem.AvgFee10
				return sharedAvgFee10, nil
			}
		}
		return 0, nil
	} else {
		if sharedAvgFee10 != 0 {
			return sharedAvgFee10, nil
		}
		return 0, nil
	}
}


func getAddressByPrikey(privateKeyHex string) string {
	wif, err := btcutil.DecodeWIF(privateKeyHex)
	if err != nil {
		log.Fatalf("Failed to decode WIF: %v", err)
		return ""
	}

	taprootAddr, err := btcutil.NewAddressTaproot(
		schnorr.SerializePubKey(txscript.ComputeTaprootKeyNoScript(wif.PrivKey.PubKey())),
		&chaincfg.MainNetParams)

	// 生成比特币地址（主网）
 	if err != nil {
		log.Printf("Error creating address: %v", err)
		return ""
	}

	return taprootAddr.String() 
}

func sendRawTransaction(txHex string) (string, error) {
	url := "http://btc:btc@127.0.0.1:8332/"
	reqBody, err := json.Marshal(map[string]interface{}{
		"jsonrpc": "1.0",
		"id":      txHex,
		"method":  "sendrawtransaction",
		"params":  []interface{}{txHex},
	})
	if err != nil {
		return "", err
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// 读取并打印响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// log.Printf("Request txHex: %s", txHex) // 打印请求的交易数据
	// log.Printf("Response body: %s", body) // 打印响应体

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	if errMsg, ok := result["error"]; ok && errMsg != nil {
		return "", fmt.Errorf("error: %v", errMsg)
	}

	txID, ok := result["result"].(string)
	if !ok {
		return "", fmt.Errorf("failed to retrieve transaction ID")
	}

	return txID, nil
}

func mint(mintNum int, myaddr string, minterPrikey string, ordinals string, gas_fee int64) {
	if mintNum > 2300 {
		log.Printf("最大2300张")
		return
	}

	mintaddr := getAddressByPrikey(minterPrikey) 
	log.Printf("mintaddr是: %s", mintaddr)

	if mintaddr == ""  {
		log.Printf("mint地址为空")
		return
	}
	
	utxo, err := getUtxoByAddress(mintaddr)
	log.Printf("utxo是Txid: %s", utxo.TxID)
	if err != nil { 
		log.Fatalf("Error inscribing: %v", err)  
	}

	if (gas_fee == 0) {
		gas_fee, err = fetchAvgFee() 
		if err != nil {
			log.Fatalf("Error: %v", err) 
		}
	}

	//构建输入utxo
	commitTxPrevOutputList := make([]*PrevOutput, 0)
	commitTxPrevOutputList = append(commitTxPrevOutputList, &PrevOutput{
		TxId:       utxo.TxID,
		VOut:       utxo.VOut,
		Amount:     utxo.Value,
		Address:    mintaddr,
		PrivateKey: minterPrikey,
	})
 
	inscriptionDataList := make([]InscriptionData, 0)
	for i := 0; i < int(mintNum); i++ {
		inscriptionDataList = append(inscriptionDataList, InscriptionData{
			ContentType: "text/plain;charset=utf-8",
			Body:        []byte(ordinals),
			RevealAddr:  myaddr,
		})
	}

	request := &InscriptionRequest{
		CommitTxPrevOutputList: commitTxPrevOutputList,
		CommitFeeRate:          gas_fee, 
		RevealFeeRate:          gas_fee,
		RevealOutValue:         330,
		InscriptionDataList:    inscriptionDataList,
		ChangeAddress:          mintaddr,
	}
	 
	txs, err := Inscribe(&chaincfg.MainNetParams, request)
	if (txs.CommitTx == "") {
		log.Printf("余额不足")
		return
	} 
	log.Printf(txs.CommitTx)

	if err != nil {
		log.Fatalf("Error inscribing: %v", err) 
	} else {
		// 广播 commitTx
		commitTxID, err := sendRawTransaction(txs.CommitTx)
		if err != nil {
			log.Fatalf("Error broadcasting commit transaction: %v", err)
		}
		log.Printf("Broadcasted commit transaction ID: %s", commitTxID)

 		// 等待支付订单先确认
		for {
			log.Printf("等待转账确认, transaction ID: %s", commitTxID)
			
			time.Sleep(10 * time.Second) // 可以根据需要调整时间间隔	
			confirmNum, _ := CheckTransactionConfirmation(commitTxID)
 			if confirmNum {
				break // 交易已确认，退出循环
			} else {
				log.Printf("等待转账确认, transaction ID: %s", commitTxID)
			}
		}

		// 广播 revealTxs 
		for _, revealTx := range txs.RevealTxs {
			revealTxID, err := sendRawTransaction(revealTx)
			if err != nil {
				log.Fatalf("Error broadcasting reveal transaction: %v", err)
			}
			log.Printf("Broadcasted reveal transaction ID: %s", revealTxID)
		}

		log.Printf("MINT订单成功上链")

		return 
	}
}


func main() {
	//mint张数 	
	mintNum := 2000
	//接收铭文的地址
	myaddr := ""
	//mint铭文付钱的地址的私钥
	minterPrikey := ""
	//打的铭文字符串
	// ordinals := `{"p":"brc-20","op":"mint","tick":"MoonCats","amt":"1000"}`
	ordinals := `{"p":"brc-20","op":"mint","tick":"MoonRats","amt":"1000000"}`

	
	//gas_fee费用
	var gas_fee int64 = 1
	for {
		mint(mintNum, myaddr, minterPrikey, ordinals, gas_fee)
		log.Printf("等待10秒")
		time.Sleep(10 * time.Second) 
	}
}
