package host

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

type ToolCaller interface {
	Call(ctx context.Context, name string, args json.RawMessage) (json.RawMessage, error)
}

type rpcReq struct {
	Tool string          `json:"tool"`
	Args json.RawMessage `json:"args"`
}

type rpcResp struct {
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}

func Register(ctx context.Context, r wazero.Runtime, caller ToolCaller) error {
	builder := r.NewHostModuleBuilder("solomon")
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, inPtr, inLen, outPtr, outCap uint32) uint32 {
			return hostRPC(ctx, m, caller, inPtr, inLen, outPtr, outCap)
		}).
		Export("rpc")
	_, err := builder.Instantiate(ctx)
	return err
}

func hostRPC(ctx context.Context, m api.Module, caller ToolCaller, inPtr, inLen, outPtr, outCap uint32) uint32 {
	mem := m.Memory()
	if mem == nil {
		return ^uint32(0)
	}
	reqBytes, ok := mem.Read(inPtr, inLen)
	if !ok {
		return ^uint32(0)
	}
	var req rpcReq
	if err := json.Unmarshal(reqBytes, &req); err != nil {
		return writeRPCError(mem, outPtr, outCap, err)
	}
	result, err := caller.Call(ctx, req.Tool, req.Args)
	if err != nil {
		return writeRPCError(mem, outPtr, outCap, err)
	}
	resp, err := json.Marshal(rpcResp{OK: true, Result: result})
	if err != nil {
		return writeRPCError(mem, outPtr, outCap, err)
	}
	return writeOut(mem, outPtr, outCap, resp)
}

func writeRPCError(mem api.Memory, outPtr, outCap uint32, err error) uint32 {
	resp, _ := json.Marshal(rpcResp{OK: false, Error: err.Error()})
	return writeOut(mem, outPtr, outCap, resp)
}

func writeOut(mem api.Memory, outPtr, outCap uint32, b []byte) uint32 {
	if uint32(len(b)) > outCap {
		return ^uint32(0)
	}
	if !mem.Write(outPtr, b) {
		return ^uint32(0)
	}
	return uint32(len(b))
}

func FormatRunError(err error) string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf("%v", err)
}
