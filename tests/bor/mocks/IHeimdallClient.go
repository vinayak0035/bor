// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/ethereum/go-ethereum/consensus/bor (interfaces: IHeimdallClient)

// Package mocks is a generated GoMock package.
package mocks

import (
	context "context"
	reflect "reflect"

	clerk "github.com/ethereum/go-ethereum/consensus/bor/clerk"
	checkpoint "github.com/ethereum/go-ethereum/consensus/bor/heimdall/checkpoint"
	milestone "github.com/ethereum/go-ethereum/consensus/bor/heimdall/milestone"
	span "github.com/ethereum/go-ethereum/consensus/bor/heimdall/span"
	gomock "github.com/golang/mock/gomock"
)

// MockIHeimdallClient is a mock of IHeimdallClient interface.
type MockIHeimdallClient struct {
	ctrl     *gomock.Controller
	recorder *MockIHeimdallClientMockRecorder
}

// MockIHeimdallClientMockRecorder is the mock recorder for MockIHeimdallClient.
type MockIHeimdallClientMockRecorder struct {
	mock *MockIHeimdallClient
}

// NewMockIHeimdallClient creates a new mock instance.
func NewMockIHeimdallClient(ctrl *gomock.Controller) *MockIHeimdallClient {
	mock := &MockIHeimdallClient{ctrl: ctrl}
	mock.recorder = &MockIHeimdallClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockIHeimdallClient) EXPECT() *MockIHeimdallClientMockRecorder {
	return m.recorder
}

// Close mocks base method.
func (m *MockIHeimdallClient) Close() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Close")
}

// Close indicates an expected call of Close.
func (mr *MockIHeimdallClientMockRecorder) Close() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Close", reflect.TypeOf((*MockIHeimdallClient)(nil).Close))
}

// FetchCheckpoint mocks base method.
func (m *MockIHeimdallClient) FetchCheckpoint(arg0 context.Context, arg1 int64) (*checkpoint.Checkpoint, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "FetchCheckpoint", arg0, arg1)
	ret0, _ := ret[0].(*checkpoint.Checkpoint)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// FetchCheckpoint indicates an expected call of FetchCheckpoint.
func (mr *MockIHeimdallClientMockRecorder) FetchCheckpoint(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FetchCheckpoint", reflect.TypeOf((*MockIHeimdallClient)(nil).FetchCheckpoint), arg0, arg1)
}

// FetchCheckpointCount mocks base method.
func (m *MockIHeimdallClient) FetchCheckpointCount(arg0 context.Context) (int64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "FetchCheckpointCount", arg0)
	ret0, _ := ret[0].(int64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// FetchCheckpointCount indicates an expected call of FetchCheckpointCount.
func (mr *MockIHeimdallClientMockRecorder) FetchCheckpointCount(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FetchCheckpointCount", reflect.TypeOf((*MockIHeimdallClient)(nil).FetchCheckpointCount), arg0)
}

// FetchLastNoAckMilestone mocks base method.
func (m *MockIHeimdallClient) FetchLastNoAckMilestone(arg0 context.Context) (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "FetchLastNoAckMilestone", arg0)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// FetchLastNoAckMilestone indicates an expected call of FetchLastNoAckMilestone.
func (mr *MockIHeimdallClientMockRecorder) FetchLastNoAckMilestone(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FetchLastNoAckMilestone", reflect.TypeOf((*MockIHeimdallClient)(nil).FetchLastNoAckMilestone), arg0)
}

// FetchMilestone mocks base method.
func (m *MockIHeimdallClient) FetchMilestone(arg0 context.Context) (*milestone.Milestone, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "FetchMilestone", arg0)
	ret0, _ := ret[0].(*milestone.Milestone)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// FetchMilestone indicates an expected call of FetchMilestone.
func (mr *MockIHeimdallClientMockRecorder) FetchMilestone(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FetchMilestone", reflect.TypeOf((*MockIHeimdallClient)(nil).FetchMilestone), arg0)
}

// FetchMilestoneCount mocks base method.
func (m *MockIHeimdallClient) FetchMilestoneCount(arg0 context.Context) (int64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "FetchMilestoneCount", arg0)
	ret0, _ := ret[0].(int64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// FetchMilestoneCount indicates an expected call of FetchMilestoneCount.
func (mr *MockIHeimdallClientMockRecorder) FetchMilestoneCount(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FetchMilestoneCount", reflect.TypeOf((*MockIHeimdallClient)(nil).FetchMilestoneCount), arg0)
}

// FetchMilestoneID mocks base method.
func (m *MockIHeimdallClient) FetchMilestoneID(arg0 context.Context, arg1 string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "FetchMilestoneID", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// FetchMilestoneID indicates an expected call of FetchMilestoneID.
func (mr *MockIHeimdallClientMockRecorder) FetchMilestoneID(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FetchMilestoneID", reflect.TypeOf((*MockIHeimdallClient)(nil).FetchMilestoneID), arg0, arg1)
}

// FetchNoAckMilestone mocks base method.
func (m *MockIHeimdallClient) FetchNoAckMilestone(arg0 context.Context, arg1 string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "FetchNoAckMilestone", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// FetchNoAckMilestone indicates an expected call of FetchNoAckMilestone.
func (mr *MockIHeimdallClientMockRecorder) FetchNoAckMilestone(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FetchNoAckMilestone", reflect.TypeOf((*MockIHeimdallClient)(nil).FetchNoAckMilestone), arg0, arg1)
}

// Span mocks base method.
func (m *MockIHeimdallClient) Span(arg0 context.Context, arg1 uint64) (*span.HeimdallSpan, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Span", arg0, arg1)
	ret0, _ := ret[0].(*span.HeimdallSpan)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Span indicates an expected call of Span.
func (mr *MockIHeimdallClientMockRecorder) Span(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Span", reflect.TypeOf((*MockIHeimdallClient)(nil).Span), arg0, arg1)
}

// StateSyncEvents mocks base method.
func (m *MockIHeimdallClient) StateSyncEvents(arg0 context.Context, arg1 uint64, arg2 int64) ([]*clerk.EventRecordWithTime, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StateSyncEvents", arg0, arg1, arg2)
	ret0, _ := ret[0].([]*clerk.EventRecordWithTime)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// StateSyncEvents indicates an expected call of StateSyncEvents.
func (mr *MockIHeimdallClientMockRecorder) StateSyncEvents(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StateSyncEvents", reflect.TypeOf((*MockIHeimdallClient)(nil).StateSyncEvents), arg0, arg1, arg2)
}
