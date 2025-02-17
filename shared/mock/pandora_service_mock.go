// Code generated by MockGen. DO NOT EDIT.
// Source: ../../validator/pandora/service.go

// Package mock is a generated GoMock package.
package mock

import (
	context "context"
	reflect "reflect"

	common "github.com/ethereum/go-ethereum/common"
	types "github.com/ethereum/go-ethereum/core/types"
	gomock "github.com/golang/mock/gomock"
	pandora "github.com/prysmaticlabs/prysm/validator/pandora"
)

// MockPandoraService is a mock of PandoraService interface
type MockPandoraService struct {
	ctrl     *gomock.Controller
	recorder *MockPandoraServiceMockRecorder
}

// MockPandoraServiceMockRecorder is the mock recorder for MockPandoraService
type MockPandoraServiceMockRecorder struct {
	mock *MockPandoraService
}

// NewMockPandoraService creates a new mock instance
func NewMockPandoraService(ctrl *gomock.Controller) *MockPandoraService {
	mock := &MockPandoraService{ctrl: ctrl}
	mock.recorder = &MockPandoraServiceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockPandoraService) EXPECT() *MockPandoraServiceMockRecorder {
	return m.recorder
}

// GetShardBlockHeader mocks base method
func (m *MockPandoraService) GetShardBlockHeader(ctx context.Context) (*types.Header, common.Hash, *pandora.ExtraData, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetShardBlockHeader", ctx)
	ret0, _ := ret[0].(*types.Header)
	ret1, _ := ret[1].(common.Hash)
	ret2, _ := ret[2].(*pandora.ExtraData)
	ret3, _ := ret[3].(error)
	return ret0, ret1, ret2, ret3
}

// GetShardBlockHeader indicates an expected call of GetShardBlockHeader
func (mr *MockPandoraServiceMockRecorder) GetShardBlockHeader(ctx interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetShardBlockHeader", reflect.TypeOf((*MockPandoraService)(nil).GetShardBlockHeader), ctx)
}

// SubmitShardBlockHeader mocks base method
func (m *MockPandoraService) SubmitShardBlockHeader(ctx context.Context, blockNonce uint64, headerHash common.Hash, sig [96]byte) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SubmitShardBlockHeader", ctx, blockNonce, headerHash, sig)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SubmitShardBlockHeader indicates an expected call of SubmitShardBlockHeader
func (mr *MockPandoraServiceMockRecorder) SubmitShardBlockHeader(ctx, blockNonce, headerHash, sig interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SubmitShardBlockHeader", reflect.TypeOf((*MockPandoraService)(nil).SubmitShardBlockHeader), ctx, blockNonce, headerHash, sig)
}

// MockRPCClient is a mock of RPCClient interface
type MockRPCClient struct {
	ctrl     *gomock.Controller
	recorder *MockRPCClientMockRecorder
}

// MockRPCClientMockRecorder is the mock recorder for MockRPCClient
type MockRPCClientMockRecorder struct {
	mock *MockRPCClient
}

// NewMockRPCClient creates a new mock instance
func NewMockRPCClient(ctrl *gomock.Controller) *MockRPCClient {
	mock := &MockRPCClient{ctrl: ctrl}
	mock.recorder = &MockRPCClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockRPCClient) EXPECT() *MockRPCClientMockRecorder {
	return m.recorder
}

// Call mocks base method
func (m *MockRPCClient) Call(result interface{}, method string, args ...interface{}) error {
	m.ctrl.T.Helper()
	varargs := []interface{}{result, method}
	for _, a := range args {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Call", varargs...)
	ret0, _ := ret[0].(error)
	return ret0
}

// Call indicates an expected call of Call
func (mr *MockRPCClientMockRecorder) Call(result, method interface{}, args ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{result, method}, args...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Call", reflect.TypeOf((*MockRPCClient)(nil).Call), varargs...)
}
