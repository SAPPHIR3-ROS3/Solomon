//go:build !wasm || !wasip1

package sdk

func callToolRaw(name string, args []byte) ([]byte, error) {
	return nil, errRPC
}
