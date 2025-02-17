// Code generated by MockGen. DO NOT EDIT.
// Source: ../../beacon-chain/orchestrator/client.go

// Package van_mock is a generated GoMock package.
package van_mock

import (
	context "context"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	params "github.com/prysmaticlabs/prysm/shared/params"
)

// MockClient is a mock of Client interface
type MockClient struct {
	ctrl     *gomock.Controller
	recorder *MockClientMockRecorder
}

// MockClientMockRecorder is the mock recorder for MockClient
type MockClientMockRecorder struct {
	mock *MockClient
}

// NewMockClient creates a new mock instance
func NewMockClient(ctrl *gomock.Controller) *MockClient {
	mock := &MockClient{ctrl: ctrl}
	mock.recorder = &MockClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockClient) EXPECT() *MockClientMockRecorder {
	return m.recorder
}

// ConfirmVanBlockHashes mocks base method
func (m *MockClient) ConfirmVanBlockHashes(arg0 context.Context, arg1 []*params.ConfirmationReqData) ([]*params.ConfirmationResData, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ConfirmVanBlockHashes", arg0, arg1)
	ret0, _ := ret[0].([]*params.ConfirmationResData)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ConfirmVanBlockHashes indicates an expected call of ConfirmVanBlockHashes
func (mr *MockClientMockRecorder) ConfirmVanBlockHashes(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ConfirmVanBlockHashes", reflect.TypeOf((*MockClient)(nil).ConfirmVanBlockHashes), arg0, arg1)
}
