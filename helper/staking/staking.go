package staking

import (
	"fmt"
	"math/big"

	"github.com/ExzoNetwork/ExzoCoin/chain"
	"github.com/ExzoNetwork/ExzoCoin/helper/common"
	"github.com/ExzoNetwork/ExzoCoin/helper/hex"
	"github.com/ExzoNetwork/ExzoCoin/helper/keccak"
	"github.com/ExzoNetwork/ExzoCoin/types"
	"github.com/ExzoNetwork/ExzoCoin/validators"
)

var (
	MinValidatorCount = uint64(1)
	MaxValidatorCount = common.MaxSafeJSInt
)

// getAddressMapping returns the key for the SC storage mapping (address => something)
//
// More information:
// https://docs.soliditylang.org/en/latest/internals/layout_in_storage.html
func getAddressMapping(address types.Address, slot int64) []byte {
	bigSlot := big.NewInt(slot)

	finalSlice := append(
		common.PadLeftOrTrim(address.Bytes(), 32),
		common.PadLeftOrTrim(bigSlot.Bytes(), 32)...,
	)

	return keccak.Keccak256(nil, finalSlice)
}

// getIndexWithOffset is a helper method for adding an offset to the already found keccak hash
func getIndexWithOffset(keccakHash []byte, offset uint64) []byte {
	bigOffset := big.NewInt(int64(offset))
	bigKeccak := big.NewInt(0).SetBytes(keccakHash)

	bigKeccak.Add(bigKeccak, bigOffset)

	return bigKeccak.Bytes()
}

// getStorageIndexes is a helper function for getting the correct indexes
// of the storage slots which need to be modified during bootstrap.
//
// It is SC dependant, and based on the SC located at:
// https://github.com/0xPolygon/staking-contracts/
func getStorageIndexes(validator validators.Validator, index int) *StorageIndexes {
	storageIndexes := &StorageIndexes{}
	address := validator.Addr()

	// Get the indexes for the mappings
	// The index for the mapping is retrieved with:
	// keccak(address . slot)
	// . stands for concatenation (basically appending the bytes)
	storageIndexes.AddressToIsValidatorIndex = getAddressMapping(
		address,
		addressToIsValidatorSlot,
	)

	storageIndexes.AddressToStakedAmountIndex = getAddressMapping(
		address,
		addressToStakedAmountSlot,
	)

	storageIndexes.AddressToValidatorIndexIndex = getAddressMapping(
		address,
		addressToValidatorIndexSlot,
	)

	storageIndexes.ValidatorBLSPublicKeyIndex = getAddressMapping(
		address,
		addressToBLSPublicKeySlot,
	)

	// Index for array types is calculated as keccak(slot) + index
	// The slot for the dynamic arrays that's put in the keccak needs to be in hex form (padded 64 chars)
	storageIndexes.ValidatorsIndex = getIndexWithOffset(
		keccak.Keccak256(nil, common.PadLeftOrTrim(big.NewInt(validatorsSlot).Bytes(), 32)),
		uint64(index),
	)

	return storageIndexes
}

// setBytesToStorage sets bytes data into storage map from specified base index
func setBytesToStorage(
	storageMap map[types.Hash]types.Hash,
	baseIndexBytes []byte,
	data []byte,
) {
	dataLen := len(data)
	baseIndex := types.BytesToHash(baseIndexBytes)

	if dataLen <= 31 {
		bytes := types.Hash{}

		copy(bytes[:len(data)], data)

		// Set 2*Size at the first byte
		bytes[len(bytes)-1] = byte(dataLen * 2)

		storageMap[baseIndex] = bytes

		return
	}

	// Set size at the base index
	baseSlot := types.Hash{}
	baseSlot[31] = byte(2*dataLen + 1)
	storageMap[baseIndex] = baseSlot

	zeroIndex := keccak.Keccak256(nil, baseIndexBytes)
	numBytesInSlot := 256 / 8

	for i := 0; i < dataLen; i++ {
		offset := i / numBytesInSlot

		slotIndex := types.BytesToHash(getIndexWithOffset(zeroIndex, uint64(offset)))
		byteIndex := i % numBytesInSlot

		slot := storageMap[slotIndex]
		slot[byteIndex] = data[i]

		storageMap[slotIndex] = slot
	}
}

// PredeployParams contains the values used to predeploy the PoS staking contract
type PredeployParams struct {
	MinValidatorCount uint64
	MaxValidatorCount uint64
}

// StorageIndexes is a wrapper for different storage indexes that
// need to be modified
type StorageIndexes struct {
	ValidatorsIndex              []byte // []address
	ValidatorBLSPublicKeyIndex   []byte // mapping(address => byte[])
	AddressToIsValidatorIndex    []byte // mapping(address => bool)
	AddressToStakedAmountIndex   []byte // mapping(address => uint256)
	AddressToValidatorIndexIndex []byte // mapping(address => uint256)
}

// Slot definitions for SC storage
var (
	validatorsSlot              = int64(0) // Slot 0
	addressToIsValidatorSlot    = int64(1) // Slot 1
	addressToStakedAmountSlot   = int64(2) // Slot 2
	addressToValidatorIndexSlot = int64(3) // Slot 3
	stakedAmountSlot            = int64(4) // Slot 4
	minNumValidatorSlot         = int64(5) // Slot 5
	maxNumValidatorSlot         = int64(6) // Slot 6
	addressToBLSPublicKeySlot   = int64(7) // Slot 7
)

const (
	DefaultStakedBalance = "0x21E19E0C9BAB2400000" // 10,000 ETH
	//nolint: lll
	StakingSCBytecode = "0x60806040523480156200001157600080fd5b5060405162001f9438038062001f948339818101604052810190620000379190620000aa565b808211156200007d576040517f08c379a0000000000000000000000000000000000000000000000000000000008152600401620000749062000118565b60405180910390fd5b81600581905550806006819055505050620001c3565b600081519050620000a481620001a9565b92915050565b60008060408385031215620000c457620000c362000155565b5b6000620000d48582860162000093565b9250506020620000e78582860162000093565b9150509250929050565b6000620001006040836200013a565b91506200010d826200015a565b604082019050919050565b600060208201905081810360008301526200013381620000f1565b9050919050565b600082825260208201905092915050565b6000819050919050565b600080fd5b7f4d696e2076616c696461746f7273206e756d2063616e206e6f7420626520677260008201527f6561746572207468616e206d6178206e756d206f662076616c696461746f7273602082015250565b620001b4816200014b565b8114620001c057600080fd5b50565b611dc180620001d36000396000f3fe6080604052600436106101185760003560e01c80637a6eea37116100a0578063d94c111b11610064578063d94c111b1461040a578063e387a7ed14610433578063e804fbf61461045e578063f90ecacc14610489578063facd743b146104c657610186565b80637a6eea37146103215780637dceceb81461034c578063af6da36e14610389578063c795c077146103b4578063ca1e7819146103df57610186565b8063373d6132116100e7578063373d6132146102595780633a4b66f1146102845780633c561f041461028e57806351a9ab32146102b9578063714ff425146102f657610186565b806302b751991461018b578063065ae171146101c85780632367f6b5146102055780632def66201461024257610186565b366101865761013c3373ffffffffffffffffffffffffffffffffffffffff16610503565b1561017c576040517f08c379a0000000000000000000000000000000000000000000000000000000008152600401610173906117dc565b60405180910390fd5b610184610516565b005b600080fd5b34801561019757600080fd5b506101b260048036038101906101ad91906113d2565b6105ed565b6040516101bf9190611837565b60405180910390f35b3480156101d457600080fd5b506101ef60048036038101906101ea91906113d2565b610605565b6040516101fc919061173f565b60405180910390f35b34801561021157600080fd5b5061022c600480360381019061022791906113d2565b610625565b6040516102399190611837565b60405180910390f35b34801561024e57600080fd5b5061025761066e565b005b34801561026557600080fd5b5061026e610759565b60405161027b9190611837565b60405180910390f35b61028c610763565b005b34801561029a57600080fd5b506102a36107cc565b6040516102b0919061171d565b60405180910390f35b3480156102c557600080fd5b506102e060048036038101906102db91906113d2565b610972565b6040516102ed919061175a565b60405180910390f35b34801561030257600080fd5b5061030b610a12565b6040516103189190611837565b60405180910390f35b34801561032d57600080fd5b50610336610a1c565b604051610343919061181c565b60405180910390f35b34801561035857600080fd5b50610373600480360381019061036e91906113d2565b610a2a565b6040516103809190611837565b60405180910390f35b34801561039557600080fd5b5061039e610a42565b6040516103ab9190611837565b60405180910390f35b3480156103c057600080fd5b506103c9610a48565b6040516103d69190611837565b60405180910390f35b3480156103eb57600080fd5b506103f4610a4e565b60405161040191906116fb565b60405180910390f35b34801561041657600080fd5b50610431600480360381019061042c91906113ff565b610adc565b005b34801561043f57600080fd5b50610448610b81565b6040516104559190611837565b60405180910390f35b34801561046a57600080fd5b50610473610b87565b6040516104809190611837565b60405180910390f35b34801561049557600080fd5b506104b060048036038101906104ab9190611448565b610b91565b6040516104bd91906116e0565b60405180910390f35b3480156104d257600080fd5b506104ed60048036038101906104e891906113d2565b610bd0565b6040516104fa919061173f565b60405180910390f35b600080823b905060008111915050919050565b34600460008282546105289190611958565b9250508190555034600260003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020600082825461057e9190611958565b9250508190555061058e33610c26565b1561059d5761059c33610ca0565b5b3373ffffffffffffffffffffffffffffffffffffffff167f9e71bc8eea02a63969f509818f2dafb9254532904319f9dbda79b67bd34a5f3d346040516105e39190611837565b60405180910390a2565b60036020528060005260406000206000915090505481565b60016020528060005260406000206000915054906101000a900460ff1681565b6000600260008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020549050919050565b61068d3373ffffffffffffffffffffffffffffffffffffffff16610503565b156106cd576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004016106c4906117dc565b60405180910390fd5b6000600260003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020541161074f576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004016107469061177c565b60405180910390fd5b610757610def565b565b6000600454905090565b6107823373ffffffffffffffffffffffffffffffffffffffff16610503565b156107c2576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004016107b9906117dc565b60405180910390fd5b6107ca610516565b565b60606000808054905067ffffffffffffffff8111156107ee576107ed611bf0565b5b60405190808252806020026020018201604052801561082157816020015b606081526020019060019003908161080c5790505b50905060005b60008054905081101561096a576007600080838154811061084b5761084a611bc1565b5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002080546108bb90611a88565b80601f01602080910402602001604051908101604052809291908181526020018280546108e790611a88565b80156109345780601f1061090957610100808354040283529160200191610934565b820191906000526020600020905b81548152906001019060200180831161091757829003601f168201915b505050505082828151811061094c5761094b611bc1565b5b6020026020010181905250808061096290611aeb565b915050610827565b508091505090565b6007602052806000526040600020600091509050805461099190611a88565b80601f01602080910402602001604051908101604052809291908181526020018280546109bd90611a88565b8015610a0a5780601f106109df57610100808354040283529160200191610a0a565b820191906000526020600020905b8154815290600101906020018083116109ed57829003601f168201915b505050505081565b6000600554905090565b69021e19e0c9bab240000081565b60026020528060005260406000206000915090505481565b60065481565b60055481565b60606000805480602002602001604051908101604052809291908181526020018280548015610ad257602002820191906000526020600020905b8160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019060010190808311610a88575b5050505050905090565b80600760003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000209080519060200190610b2f929190611295565b503373ffffffffffffffffffffffffffffffffffffffff167f472da4d064218fa97032725fbcff922201fa643fed0765b5ffe0ceef63d7b3dc82604051610b76919061175a565b60405180910390a250565b60045481565b6000600654905090565b60008181548110610ba157600080fd5b906000526020600020016000915054906101000a900473ffffffffffffffffffffffffffffffffffffffff1681565b6000600160008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060009054906101000a900460ff169050919050565b6000610c3182610f41565b158015610c99575069021e19e0c9bab24000006fffffffffffffffffffffffffffffffff16600260008473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000205410155b9050919050565b60065460008054905010610ce9576040517f08c379a0000000000000000000000000000000000000000000000000000000008152600401610ce09061179c565b60405180910390fd5b60018060008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060006101000a81548160ff021916908315150217905550600080549050600360008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020819055506000819080600181540180825580915050600190039060005260206000200160009091909190916101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555050565b6000600260003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000205490506000600260003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020819055508060046000828254610e8a91906119ae565b92505081905550610e9a33610f41565b15610ea957610ea833610f97565b5b3373ffffffffffffffffffffffffffffffffffffffff166108fc829081150290604051600060405180830381858888f19350505050158015610eef573d6000803e3d6000fd5b503373ffffffffffffffffffffffffffffffffffffffff167f0f5bb82176feb1b5e747e28471aa92156a04d9f3ab9f45f28e2d704232b93f7582604051610f369190611837565b60405180910390a250565b6000600160008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060009054906101000a900460ff169050919050565b60055460008054905011610fe0576040517f08c379a0000000000000000000000000000000000000000000000000000000008152600401610fd7906117fc565b60405180910390fd5b600080549050600360008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000205410611066576040517f08c379a000000000000000000000000000000000000000000000000000000000815260040161105d906117bc565b60405180910390fd5b6000600360008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020549050600060016000805490506110be91906119ae565b90508082146111ac5760008082815481106110dc576110db611bc1565b5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff169050806000848154811061111e5761111d611bc1565b5b9060005260206000200160006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555082600360008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002081905550505b6000600160008573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060006101000a81548160ff0219169083151502179055506000600360008573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002081905550600080548061125b5761125a611b92565b5b6001900381819060005260206000200160006101000a81549073ffffffffffffffffffffffffffffffffffffffff02191690559055505050565b8280546112a190611a88565b90600052602060002090601f0160209004810192826112c3576000855561130a565b82601f106112dc57805160ff191683800117855561130a565b8280016001018555821561130a579182015b828111156113095782518255916020019190600101906112ee565b5b509050611317919061131b565b5090565b5b8082111561133457600081600090555060010161131c565b5090565b600061134b61134684611877565b611852565b90508281526020810184848401111561136757611366611c24565b5b611372848285611a46565b509392505050565b60008135905061138981611d5d565b92915050565b600082601f8301126113a4576113a3611c1f565b5b81356113b4848260208601611338565b91505092915050565b6000813590506113cc81611d74565b92915050565b6000602082840312156113e8576113e7611c2e565b5b60006113f68482850161137a565b91505092915050565b60006020828403121561141557611414611c2e565b5b600082013567ffffffffffffffff81111561143357611432611c29565b5b61143f8482850161138f565b91505092915050565b60006020828403121561145e5761145d611c2e565b5b600061146c848285016113bd565b91505092915050565b600061148183836114a1565b60208301905092915050565b600061149983836115a1565b905092915050565b6114aa816119e2565b82525050565b6114b9816119e2565b82525050565b60006114ca826118c8565b6114d48185611903565b93506114df836118a8565b8060005b838110156115105781516114f78882611475565b9750611502836118e9565b9250506001810190506114e3565b5085935050505092915050565b6000611528826118d3565b6115328185611914565b935083602082028501611544856118b8565b8060005b858110156115805784840389528151611561858261148d565b945061156c836118f6565b925060208a01995050600181019050611548565b50829750879550505050505092915050565b61159b816119f4565b82525050565b60006115ac826118de565b6115b68185611925565b93506115c6818560208601611a55565b6115cf81611c33565b840191505092915050565b60006115e5826118de565b6115ef8185611936565b93506115ff818560208601611a55565b61160881611c33565b840191505092915050565b6000611620601d83611947565b915061162b82611c44565b602082019050919050565b6000611643602783611947565b915061164e82611c6d565b604082019050919050565b6000611666601283611947565b915061167182611cbc565b602082019050919050565b6000611689601a83611947565b915061169482611ce5565b602082019050919050565b60006116ac604083611947565b91506116b782611d0e565b604082019050919050565b6116cb81611a00565b82525050565b6116da81611a3c565b82525050565b60006020820190506116f560008301846114b0565b92915050565b6000602082019050818103600083015261171581846114bf565b905092915050565b60006020820190508181036000830152611737818461151d565b905092915050565b60006020820190506117546000830184611592565b92915050565b6000602082019050818103600083015261177481846115da565b905092915050565b6000602082019050818103600083015261179581611613565b9050919050565b600060208201905081810360008301526117b581611636565b9050919050565b600060208201905081810360008301526117d581611659565b9050919050565b600060208201905081810360008301526117f58161167c565b9050919050565b600060208201905081810360008301526118158161169f565b9050919050565b600060208201905061183160008301846116c2565b92915050565b600060208201905061184c60008301846116d1565b92915050565b600061185c61186d565b90506118688282611aba565b919050565b6000604051905090565b600067ffffffffffffffff82111561189257611891611bf0565b5b61189b82611c33565b9050602081019050919050565b6000819050602082019050919050565b6000819050602082019050919050565b600081519050919050565b600081519050919050565b600081519050919050565b6000602082019050919050565b6000602082019050919050565b600082825260208201905092915050565b600082825260208201905092915050565b600082825260208201905092915050565b600082825260208201905092915050565b600082825260208201905092915050565b600061196382611a3c565b915061196e83611a3c565b9250827fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff038211156119a3576119a2611b34565b5b828201905092915050565b60006119b982611a3c565b91506119c483611a3c565b9250828210156119d7576119d6611b34565b5b828203905092915050565b60006119ed82611a1c565b9050919050565b60008115159050919050565b60006fffffffffffffffffffffffffffffffff82169050919050565b600073ffffffffffffffffffffffffffffffffffffffff82169050919050565b6000819050919050565b82818337600083830152505050565b60005b83811015611a73578082015181840152602081019050611a58565b83811115611a82576000848401525b50505050565b60006002820490506001821680611aa057607f821691505b60208210811415611ab457611ab3611b63565b5b50919050565b611ac382611c33565b810181811067ffffffffffffffff82111715611ae257611ae1611bf0565b5b80604052505050565b6000611af682611a3c565b91507fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff821415611b2957611b28611b34565b5b600182019050919050565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052601160045260246000fd5b7f4e487b7100000000000000000000000000000000000000000000000000000000600052602260045260246000fd5b7f4e487b7100000000000000000000000000000000000000000000000000000000600052603160045260246000fd5b7f4e487b7100000000000000000000000000000000000000000000000000000000600052603260045260246000fd5b7f4e487b7100000000000000000000000000000000000000000000000000000000600052604160045260246000fd5b600080fd5b600080fd5b600080fd5b600080fd5b6000601f19601f8301169050919050565b7f4f6e6c79207374616b65722063616e2063616c6c2066756e6374696f6e000000600082015250565b7f56616c696461746f72207365742068617320726561636865642066756c6c206360008201527f6170616369747900000000000000000000000000000000000000000000000000602082015250565b7f696e646578206f7574206f662072616e67650000000000000000000000000000600082015250565b7f4f6e6c7920454f412063616e2063616c6c2066756e6374696f6e000000000000600082015250565b7f56616c696461746f72732063616e2774206265206c657373207468616e20746860008201527f65206d696e696d756d2072657175697265642076616c696461746f72206e756d602082015250565b611d66816119e2565b8114611d7157600080fd5b50565b611d7d81611a3c565b8114611d8857600080fd5b5056fea2646970667358221220190e179fb616b7a9067ce4a6e22d6ec36f9e27315e4a913c83d3739ecb55d74464736f6c63430008070033"
)

// PredeployStakingSC is a helper method for setting up the staking smart contract account,
// using the passed in validators as pre-staked validators
func PredeployStakingSC(
	vals validators.Validators,
	params PredeployParams,
) (*chain.GenesisAccount, error) {
	// Set the code for the staking smart contract
	// Code retrieved from https://github.com/0xPolygon/staking-contracts
	scHex, _ := hex.DecodeHex(StakingSCBytecode)
	stakingAccount := &chain.GenesisAccount{
		Code: scHex,
	}

	// Parse the default staked balance value into *big.Int
	val := DefaultStakedBalance
	bigDefaultStakedBalance, err := types.ParseUint256orHex(&val)

	if err != nil {
		return nil, fmt.Errorf("unable to generate DefaultStatkedBalance, %w", err)
	}

	// Generate the empty account storage map
	storageMap := make(map[types.Hash]types.Hash)
	bigTrueValue := big.NewInt(1)
	stakedAmount := big.NewInt(0)
	bigMinNumValidators := big.NewInt(int64(params.MinValidatorCount))
	bigMaxNumValidators := big.NewInt(int64(params.MaxValidatorCount))
	valsLen := big.NewInt(0)

	if vals != nil {
		valsLen = big.NewInt(int64(vals.Len()))

		for idx := 0; idx < vals.Len(); idx++ {
			validator := vals.At(uint64(idx))

			// Update the total staked amount
			stakedAmount = stakedAmount.Add(stakedAmount, bigDefaultStakedBalance)

			// Get the storage indexes
			storageIndexes := getStorageIndexes(validator, idx)

			// Set the value for the validators array
			storageMap[types.BytesToHash(storageIndexes.ValidatorsIndex)] =
				types.BytesToHash(
					validator.Addr().Bytes(),
				)

			if blsValidator, ok := validator.(*validators.BLSValidator); ok {
				setBytesToStorage(
					storageMap,
					storageIndexes.ValidatorBLSPublicKeyIndex,
					blsValidator.BLSPublicKey,
				)
			}

			// Set the value for the address -> validator array index mapping
			storageMap[types.BytesToHash(storageIndexes.AddressToIsValidatorIndex)] =
				types.BytesToHash(bigTrueValue.Bytes())

			// Set the value for the address -> staked amount mapping
			storageMap[types.BytesToHash(storageIndexes.AddressToStakedAmountIndex)] =
				types.StringToHash(hex.EncodeBig(bigDefaultStakedBalance))

			// Set the value for the address -> validator index mapping
			storageMap[types.BytesToHash(storageIndexes.AddressToValidatorIndexIndex)] =
				types.StringToHash(hex.EncodeUint64(uint64(idx)))
		}
	}

	// Set the value for the total staked amount
	storageMap[types.BytesToHash(big.NewInt(stakedAmountSlot).Bytes())] =
		types.BytesToHash(stakedAmount.Bytes())

	// Set the value for the size of the validators array
	storageMap[types.BytesToHash(big.NewInt(validatorsSlot).Bytes())] =
		types.BytesToHash(valsLen.Bytes())

	// Set the value for the minimum number of validators
	storageMap[types.BytesToHash(big.NewInt(minNumValidatorSlot).Bytes())] =
		types.BytesToHash(bigMinNumValidators.Bytes())

	// Set the value for the maximum number of validators
	storageMap[types.BytesToHash(big.NewInt(maxNumValidatorSlot).Bytes())] =
		types.BytesToHash(bigMaxNumValidators.Bytes())

	// Save the storage map
	stakingAccount.Storage = storageMap

	// Set the Staking SC balance to numValidators * defaultStakedBalance
	stakingAccount.Balance = stakedAmount

	return stakingAccount, nil
}
