// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/hashicorp/raft (interfaces: ApplyFuture)

// Package raftleaseconsumer is a generated GoMock package.
package raftleaseconsumer

import (
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
)

// MockApplyFuture is a mock of ApplyFuture interface.
type MockApplyFuture struct {
	ctrl     *gomock.Controller
	recorder *MockApplyFutureMockRecorder
}

// MockApplyFutureMockRecorder is the mock recorder for MockApplyFuture.
type MockApplyFutureMockRecorder struct {
	mock *MockApplyFuture
}

// NewMockApplyFuture creates a new mock instance.
func NewMockApplyFuture(ctrl *gomock.Controller) *MockApplyFuture {
	mock := &MockApplyFuture{ctrl: ctrl}
	mock.recorder = &MockApplyFutureMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockApplyFuture) EXPECT() *MockApplyFutureMockRecorder {
	return m.recorder
}

// Error mocks base method.
func (m *MockApplyFuture) Error() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Error")
	ret0, _ := ret[0].(error)
	return ret0
}

// Error indicates an expected call of Error.
func (mr *MockApplyFutureMockRecorder) Error() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Error", reflect.TypeOf((*MockApplyFuture)(nil).Error))
}

// Index mocks base method.
func (m *MockApplyFuture) Index() uint64 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Index")
	ret0, _ := ret[0].(uint64)
	return ret0
}

// Index indicates an expected call of Index.
func (mr *MockApplyFutureMockRecorder) Index() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Index", reflect.TypeOf((*MockApplyFuture)(nil).Index))
}

// Response mocks base method.
func (m *MockApplyFuture) Response() interface{} {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Response")
	ret0, _ := ret[0].(interface{})
	return ret0
}

// Response indicates an expected call of Response.
func (mr *MockApplyFutureMockRecorder) Response() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Response", reflect.TypeOf((*MockApplyFuture)(nil).Response))
}
