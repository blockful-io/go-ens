package ens

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/wealdtech/go-ens/v3/contracts/offchainresolver"
	"github.com/wealdtech/go-ens/v3/contracts/universalresolver"
)

func CCIPRead(backend bind.ContractBackend, rAddr common.Address, revertData string) ([]byte, error) {
	hexBytes, err := hex.DecodeString(revertData[2:])
	if err != nil {
		return nil, err
	}

	uAbi, err := universalresolver.ContractMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	var sig [4]byte
	copy(sig[:], hexBytes[:4])
	abiErr, err := uAbi.ErrorByID(sig)
	if err != nil {
		return nil, err
	}
	errArgs, err := abiErr.Inputs.Unpack(hexBytes[4:])
	if err != nil {
		return nil, err
	}

	sender := errArgs[0].(common.Address)
	urls := errArgs[1].([]string)
	calldata := common.Bytes2Hex(errArgs[2].([]byte))
	calldataHex := fmt.Sprintf("0x%s", calldata)
	callback := errArgs[3].([4]byte)
	extraData := errArgs[4].([]byte)

	resp, err := CCIPFetch(sender, calldataHex, urls)
	if err != nil || len(resp) == 0 {
		return nil, errors.New("unregistered name")
	}

	oAbi, err := offchainresolver.ContractMetaData.GetAbi()
	if err != nil {
		return nil, errors.New("no address")
	}
	m, err := oAbi.MethodById(callback[:])
	if err != nil {
		return nil, errors.New("no address")
	}
	args, err := m.Inputs.Pack(common.Hex2Bytes(resp[2:]), extraData)
	if err != nil {
		return nil, errors.New("no address")
	}

	return backend.CallContract(context.Background(), ethereum.CallMsg{
		To:   &rAddr,
		Data: append(callback[:], args...),
	}, nil)
}

func CCIPFetch(sender common.Address, data string, urls []string) (result string, err error) {
	for _, url := range urls {
		method := "POST"
		if strings.Contains(url, "{data}") {
			method = "GET"
		}

		body := []byte{}
		if method == "POST" {
			body, err = json.Marshal(map[string]interface{}{
				"data":   data,
				"sender": sender,
			})
			if err != nil {
				return "", err
			}
		}

		url = strings.ReplaceAll(url, "{sender}", sender.String())
		url = strings.ReplaceAll(url, "{data}", data)
		req, err := http.NewRequest(method, url, bytes.NewBuffer(body))
		if err != nil {
			return "", err
		}
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return "", err
		}

		defer resp.Body.Close()

		var responseData map[string]interface{}
		var result string
		if strings.HasPrefix(resp.Header.Get("Content-Type"), "application/json") {
			err = json.NewDecoder(resp.Body).Decode(&responseData)
			if err != nil {
				continue
			}
			var ok bool
			result, ok = responseData["data"].(string)
			if !ok {
				err = errors.New("invalid response from gateway")
				continue
			}
		} else {
			responseBytes, err := io.ReadAll(resp.Body)
			if err != nil {
				continue
			}
			result = string(responseBytes)
		}

		if resp.StatusCode != http.StatusOK {
			continue
		}

		return result, nil
	}

	return "", err
}
