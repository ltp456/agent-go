package agent

import (
	"bytes"
	"crypto/sha256"
	"math/big"
	"math/rand"
	"sort"

	"github.com/aviate-labs/candid-go"
	"github.com/aviate-labs/leb128"
	"github.com/aviate-labs/principal-go"
)

var (
	typeKey          = sha256.Sum256([]byte("request_type"))
	canisterIDKey    = sha256.Sum256([]byte("canister_id"))
	nonceKey         = sha256.Sum256([]byte("nonce"))
	methodNameKey    = sha256.Sum256([]byte("method_name"))
	argumentsKey     = sha256.Sum256([]byte("arg"))
	ingressExpiryKey = sha256.Sum256([]byte("ingress_expiry"))
	senderKey        = sha256.Sum256([]byte("sender"))
)

func encodeLEB128(i uint64) []byte {
	bi := big.NewInt(int64(i))
	e, _ := leb128.EncodeUnsigned(bi)
	return e
}

// DOCS: https://smartcontracts.org/docs/interface-spec/index.html#http-call
type Request struct {
	Type RequestType `cbor:"request_type"`
	// The user who issued the request.
	Sender principal.Principal `cbor:"sender"`
	// Arbitrary user-provided data, typically randomly generated. This can be
	// used to create distinct requests with otherwise identical fields.
	Nonce []byte `cbor:"nonce"`
	// An upper limit on the validity of the request, expressed in nanoseconds
	// since 1970-01-01 (like ic0.time()).
	IngressExpiry uint64 `cbor:"ingress_expiry"`
	// The principal of the canister to call.
	CanisterID principal.Principal `cbor:"canister_id"`
	// Name of the canister method to call.
	MethodName string `cbor:"method_name"`
	// Argument to pass to the canister method.
	Arguments []byte `cbor:"arg"`
}

func NewRequest(
	sender principal.Principal,
	requestType RequestType,
	canisterId principal.Principal,
	methodName string,
	arguments string,
	effective uint64,
) (Request, error) {
	args, err := candid.EncodeValue(arguments)
	if err != nil {
		return Request{}, nil
	}
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return Request{}, err
	}
	return Request{
		Type:          requestType,
		Sender:        sender,
		Nonce:         nonce,
		IngressExpiry: 1000000000 * (effective + 300),
		CanisterID:    canisterId,
		MethodName:    methodName,
		Arguments:     args,
	}, nil
}

type RequestID [32]byte

// DOCS: https://smartcontracts.org/docs/interface-spec/index.html#request-id
func NewRequestID(req Request) RequestID {
	var (
		typeHash       = sha256.Sum256([]byte(req.Type))
		canisterIDHash = sha256.Sum256(req.CanisterID)
		methodNameHash = sha256.Sum256([]byte(req.MethodName))
		argumentsHash  = sha256.Sum256(req.Arguments)
	)
	hashes := [][]byte{
		append(typeKey[:], typeHash[:]...),
		append(canisterIDKey[:], canisterIDHash[:]...),
		append(methodNameKey[:], methodNameHash[:]...),
		append(argumentsKey[:], argumentsHash[:]...),
	}
	if len(req.Sender) != 0 {
		senderHash := sha256.Sum256(req.Sender)
		hashes = append(hashes, append(senderKey[:], senderHash[:]...))
	}
	if req.IngressExpiry != 0 {
		ingressExpiryHash := sha256.Sum256(encodeLEB128(req.IngressExpiry))
		hashes = append(hashes, append(ingressExpiryKey[:], ingressExpiryHash[:]...))
	}
	if len(req.Nonce) != 0 {
		nonceHash := sha256.Sum256(req.Nonce)
		hashes = append(hashes, append(nonceKey[:], nonceHash[:]...))
	}
	sort.Slice(hashes, func(i, j int) bool {
		return bytes.Compare(hashes[i], hashes[j]) == -1
	})
	return sha256.Sum256(bytes.Join(hashes, nil))
}

func (r RequestID) Sign(id Identity) ([]byte, error) {
	h := sha256.Sum256(append(
		// \x0Aic-request
		[]byte{0x0a, 0x69, 0x63, 0x2d, 0x72, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74},
		r[:]...,
	))
	return id.Sign(h[:])
}

type RequestType = string

const (
	RequestTypeCall  RequestType = "call"
	RequestTypeQuery RequestType = "query"
)
