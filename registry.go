// Copyright 2017 Weald Technology Trading
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ens

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/params"
	"github.com/wealdtech/go-ens/v2/contracts/auctionregistrar"
	"github.com/wealdtech/go-ens/v2/contracts/registry"
	"github.com/wealdtech/go-ens/v2/util"
)

// Registry is the structure for the registry contract
type Registry struct {
	client   *ethclient.Client
	Contract *registry.Contract
}

// NewRegistry obtains the ENS registry
func NewRegistry(client *ethclient.Client) (*Registry, error) {
	address, err := RegistryContractAddress(client)
	if err != nil {
		return nil, err
	}
	return NewRegistryAt(client, address)
}

// NewRegistryAt obtains the ENS registry at a given address
func NewRegistryAt(client *ethclient.Client, address common.Address) (*Registry, error) {
	contract, err := registry.NewContract(address, client)
	if err != nil {
		return nil, err
	}
	return &Registry{
		client:   client,
		Contract: contract,
	}, nil
}

// Owner returns the address of the owner of a name
func (r *Registry) Owner(name string) (common.Address, error) {
	return r.Contract.Owner(nil, NameHash(name))
}

// ResolverAddress returns the address of the resolver for a name
func (r *Registry) ResolverAddress(name string) (common.Address, error) {
	return r.Contract.Resolver(nil, NameHash(name))
}

// SetResolver sets the resolver for a name
func (r *Registry) SetResolver(opts *bind.TransactOpts, name string, address common.Address) (*types.Transaction, error) {
	return r.Contract.SetResolver(opts, NameHash(name), address)
}

// Resolver returns the resolver for a name
func (r *Registry) Resolver(name string) (*Resolver, error) {
	address, err := r.ResolverAddress(name)
	if err != nil {
		return nil, err
	}
	return NewResolverAt(r.client, name, address)
}

// SetOwner sets the ownership of a domain
func (r *Registry) SetOwner(opts *bind.TransactOpts, name string, address common.Address) (*types.Transaction, error) {
	return r.Contract.SetOwner(opts, NameHash(name), address)
}

// SetSubdomainOwner sets the ownership of a subdomain, potentially creating it in the process
func (r *Registry) SetSubdomainOwner(opts *bind.TransactOpts, name string, subname string, address common.Address) (*types.Transaction, error) {
	return r.Contract.SetSubnodeOwner(opts, NameHash(name), LabelHash(subname), address)
}

// RegistryContractAddress obtains the address of the registry contract for a chain
func RegistryContractAddress(client *ethclient.Client) (common.Address, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	chainID, err := client.NetworkID(ctx)
	if err != nil {
		return UnknownAddress, err
	}

	// Instantiate the registry contract
	if chainID.Cmp(params.MainnetChainConfig.ChainID) == 0 {
		return common.HexToAddress("314159265dd8dbb310642f98f50c066173c1259b"), nil
	} else if chainID.Cmp(params.TestnetChainConfig.ChainID) == 0 {
		return common.HexToAddress("112234455c3a32fd11230c42e7bccd4a84e02010"), nil
	} else if chainID.Cmp(params.RinkebyChainConfig.ChainID) == 0 {
		return common.HexToAddress("e7410170f87102DF0055eB195163A03B7F2Bff4A"), nil
	} else if chainID.Cmp(params.GoerliChainConfig.ChainID) == 0 {
		return common.HexToAddress("112234455c3a32fd11230c42e7bccd4a84e02010"), nil
	} else {
		return UnknownAddress, fmt.Errorf("No contract for network ID %v", chainID)
	}
}

//// RegistryContract obtains the registry contract for a chain
//func RegistryContract(client *ethclient.Client) (*registry.RegistryContract, error) {
//	address, err := RegistryContractAddress(client)
//	if err != nil {
//		return nil, err
//	}
//
//	// Instantiate the registry contract
//	return registry.NewRegistryContract(address, client)
//}

// RegistryContractFromRegistrar obtains the registry contract given an
// existing registrar contract
func RegistryContractFromRegistrar(client *ethclient.Client, registrar *auctionregistrar.Contract) (*registry.RegistryContract, error) {
	if registrar == nil {
		return nil, errors.New("no registrar contract")
	}
	registryAddress, err := registrar.Ens(nil)
	if err != nil {
		return nil, err
	}
	return registry.NewRegistryContract(registryAddress, client)
}

//// Resolver obtains the address of the resolver for a .eth name
//func Resolver(contract *registry.RegistryContract, name string) (common.Address, error) {
//	if contract == nil {
//		return UnknownAddress, errors.New("no registry contract")
//	}
//	address, err := contract.Resolver(nil, NameHash(name))
//	if err == nil && bytes.Compare(address.Bytes(), UnknownAddress.Bytes()) == 0 {
//		err = errors.New("no resolver")
//	}
//	return address, err
//}

// SetResolver sets the resolver for a name
func SetResolver(session *registry.RegistryContractSession, name string, resolverAddr *common.Address) (*types.Transaction, error) {
	return session.SetResolver(NameHash(name), *resolverAddr)
}

// SetSubdomainOwner sets the owner for a subdomain of a name
func SetSubdomainOwner(session *registry.RegistryContractSession, name string, subdomain string, ownerAddr *common.Address) (*types.Transaction, error) {
	return session.SetSubnodeOwner(NameHash(name), LabelHash(subdomain), *ownerAddr)
}

// CreateRegistrySession creates a session suitable for multiple calls
func CreateRegistrySession(chainID *big.Int, wallet *accounts.Wallet, account *accounts.Account, passphrase string, contract *registry.RegistryContract, gasPrice *big.Int) *registry.RegistryContractSession {
	// Create a signer
	signer := util.AccountSigner(chainID, wallet, account, passphrase)

	// Return our session
	session := &registry.RegistryContractSession{
		Contract: contract,
		CallOpts: bind.CallOpts{
			Pending: true,
		},
		TransactOpts: bind.TransactOpts{
			From:     account.Address,
			Signer:   signer,
			GasPrice: gasPrice,
		},
	}

	return session
}
