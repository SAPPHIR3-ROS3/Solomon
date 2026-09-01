//go:build wasm && wasip1

package sdk

import "unsafe"

//go:wasmimport solomon rpc
func wasmRPC(inPtr, inLen, outPtr, outCap uint32) uint32

func slicePtr(b []byte) uint32 {
	if len(b) == 0 {
		return 0
	}
	return uint32(uintptr(unsafe.Pointer(&b[0])))
}

const maxRPCResponseBytes = 8 * 1024 * 1024

func callToolRaw(name string, args []byte) ([]byte, error) {
	req, err := marshalRPCRequest(name, args)
	if err != nil {
		return nil, err
	}
	// Tool output is truncated after the host call; keep enough room for the documented 5 MB fetch body plus JSON framing.
	out := make([]byte, maxRPCResponseBytes)
	written := wasmRPC(slicePtr(req), uint32(len(req)), slicePtr(out), uint32(len(out)))
	if written == ^uint32(0) {
		return nil, errRPCResponseTooLarge
	}
	if written == 0 {
		return nil, errRPC
	}
	return decodeRPCResult(out[:written])
}
