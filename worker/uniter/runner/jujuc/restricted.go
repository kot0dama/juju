// Copyright 2012-2014 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package jujuc

import (
	"time"

	"github.com/juju/charm/v9"
	"github.com/juju/errors"
	"github.com/juju/names/v4"

	"github.com/juju/juju/apiserver/params"
	"github.com/juju/juju/core/application"
	"github.com/juju/juju/core/network"
	"github.com/juju/juju/core/secrets"
)

// ErrRestrictedContext indicates a method is not implemented in the given context.
var ErrRestrictedContext = errors.NotImplementedf("not implemented for restricted context")

// RestrictedContext is a base implementation for restricted contexts to embed,
// so that an error is returned for methods that are not explicitly
// implemented.
type RestrictedContext struct{}

// ConfigSettings implements hooks.Context.
func (*RestrictedContext) ConfigSettings() (charm.Settings, error) { return nil, ErrRestrictedContext }

// GoalState implements hooks.Context.
func (*RestrictedContext) GoalState() (*application.GoalState, error) {
	return &application.GoalState{}, ErrRestrictedContext
}

// GetCharmState implements jujuc.unitCharmStateContext.
func (*RestrictedContext) GetCharmState() (map[string]string, error) {
	return nil, ErrRestrictedContext
}

// GetSingleCharmStateValue implements jujuc.unitCharmStateContext.
func (*RestrictedContext) GetCharmStateValue(string) (string, error) {
	return "", ErrRestrictedContext
}

// DeleteCharmStateValue implements jujuc.unitCharmStateContext.
func (*RestrictedContext) DeleteCharmStateValue(string) error {
	return ErrRestrictedContext
}

// SetCharmStateValue implements jujuc.unitCharmStateContext.
func (*RestrictedContext) SetCharmStateValue(string, string) error {
	return ErrRestrictedContext
}

// UnitStatus implements hooks.Context.
func (*RestrictedContext) UnitStatus() (*StatusInfo, error) {
	return nil, ErrRestrictedContext
}

// SetPodSpec implements hooks.Context.
func (c *RestrictedContext) SetPodSpec(specYaml string) error {
	return ErrRestrictedContext
}

// GetPodSpec implements hooks.Context.
func (c *RestrictedContext) GetPodSpec() (string, error) {
	return "", ErrRestrictedContext
}

// SetRawK8sSpec implements hooks.Context.
func (c *RestrictedContext) SetRawK8sSpec(specYaml string) error {
	return ErrRestrictedContext
}

// GetRawK8sSpec implements hooks.Context.
func (c *RestrictedContext) GetRawK8sSpec() (string, error) {
	return "", ErrRestrictedContext
}

// CloudSpec implements hooks.Context.
func (c *RestrictedContext) CloudSpec() (*params.CloudSpec, error) {
	return nil, ErrRestrictedContext
}

// SetUnitStatus implements hooks.Context.
func (*RestrictedContext) SetUnitStatus(StatusInfo) error { return ErrRestrictedContext }

// ApplicationStatus implements hooks.Context.
func (*RestrictedContext) ApplicationStatus() (ApplicationStatusInfo, error) {
	return ApplicationStatusInfo{}, ErrRestrictedContext
}

// SetApplicationStatus implements hooks.Context.
func (*RestrictedContext) SetApplicationStatus(StatusInfo) error {
	return ErrRestrictedContext
}

// AvailabilityZone implements hooks.Context.
func (*RestrictedContext) AvailabilityZone() (string, error) { return "", ErrRestrictedContext }

// RequestReboot implements hooks.Context.
func (*RestrictedContext) RequestReboot(prio RebootPriority) error {
	return ErrRestrictedContext
}

// PublicAddress implements hooks.Context.
func (*RestrictedContext) PublicAddress() (string, error) { return "", ErrRestrictedContext }

// PrivateAddress implements hooks.Context.
func (*RestrictedContext) PrivateAddress() (string, error) { return "", ErrRestrictedContext }

// OpenPortRange implements hooks.Context.
func (*RestrictedContext) OpenPortRange(string, network.PortRange) error {
	return ErrRestrictedContext
}

// ClosePortRange implements hooks.Context.
func (*RestrictedContext) ClosePortRange(string, network.PortRange) error {
	return ErrRestrictedContext
}

// OpenedPortRanges implements hooks.Context.
func (*RestrictedContext) OpenedPortRanges() network.GroupedPortRanges { return nil }

// NetworkInfo implements hooks.Context.
func (*RestrictedContext) NetworkInfo(bindingNames []string, relationId int) (map[string]params.NetworkInfoResult, error) {
	return map[string]params.NetworkInfoResult{}, ErrRestrictedContext
}

// IsLeader implements hooks.Context.
func (*RestrictedContext) IsLeader() (bool, error) { return false, ErrRestrictedContext }

// LeaderSettings implements hooks.Context.
func (*RestrictedContext) LeaderSettings() (map[string]string, error) {
	return nil, ErrRestrictedContext
}

// WriteLeaderSettings implements hooks.Context.
func (*RestrictedContext) WriteLeaderSettings(map[string]string) error { return ErrRestrictedContext }

// AddMetric implements hooks.Context.
func (*RestrictedContext) AddMetric(string, string, time.Time) error { return ErrRestrictedContext }

// AddMetricLabels implements hooks.Context.
func (*RestrictedContext) AddMetricLabels(string, string, time.Time, map[string]string) error {
	return ErrRestrictedContext
}

// StorageTags implements hooks.Context.
func (*RestrictedContext) StorageTags() ([]names.StorageTag, error) { return nil, ErrRestrictedContext }

// Storage implements hooks.Context.
func (*RestrictedContext) Storage(names.StorageTag) (ContextStorageAttachment, error) {
	return nil, ErrRestrictedContext
}

// HookStorage implements hooks.Context.
func (*RestrictedContext) HookStorage() (ContextStorageAttachment, error) {
	return nil, ErrRestrictedContext
}

// AddUnitStorage implements hooks.Context.
func (*RestrictedContext) AddUnitStorage(map[string]params.StorageConstraints) error {
	return ErrRestrictedContext
}

// Relation implements hooks.Context.
func (*RestrictedContext) Relation(id int) (ContextRelation, error) {
	return nil, ErrRestrictedContext
}

// RelationIds implements hooks.Context.
func (*RestrictedContext) RelationIds() ([]int, error) { return nil, ErrRestrictedContext }

// HookRelation implements hooks.Context.
func (*RestrictedContext) HookRelation() (ContextRelation, error) {
	return nil, ErrRestrictedContext
}

// RemoteUnitName implements hooks.Context.
func (*RestrictedContext) RemoteUnitName() (string, error) { return "", ErrRestrictedContext }

// RemoteApplicationName implements hooks.Context.
func (*RestrictedContext) RemoteApplicationName() (string, error) { return "", ErrRestrictedContext }

// ActionParams implements hooks.Context.
func (*RestrictedContext) ActionParams() (map[string]interface{}, error) {
	return nil, ErrRestrictedContext
}

// UpdateActionResults implements hooks.Context.
func (*RestrictedContext) UpdateActionResults(keys []string, value interface{}) error {
	return ErrRestrictedContext
}

// LogActionMessage implements hooks.Context.
func (*RestrictedContext) LogActionMessage(string) error { return ErrRestrictedContext }

// SetActionMessage implements hooks.Context.
func (*RestrictedContext) SetActionMessage(string) error { return ErrRestrictedContext }

// SetActionFailed implements hooks.Context.
func (*RestrictedContext) SetActionFailed() error { return ErrRestrictedContext }

// Component implements jujc.Context.
func (*RestrictedContext) Component(string) (ContextComponent, error) {
	return nil, ErrRestrictedContext
}

// UnitWorkloadVersion implements hooks.Context.
func (*RestrictedContext) UnitWorkloadVersion() (string, error) {
	return "", ErrRestrictedContext
}

// SetUnitWorkloadVersion implements hooks.Context.
func (*RestrictedContext) SetUnitWorkloadVersion(string) error {
	return ErrRestrictedContext
}

// WorkloadName implements hooks.Context.
func (*RestrictedContext) WorkloadName() (string, error) {
	return "", ErrRestrictedContext
}

// GetSecret implements runner.Context.
func (ctx *RestrictedContext) GetSecret(ID string) (secrets.SecretValue, error) {
	return nil, ErrRestrictedContext
}

// CreateSecret implements runner.Context.
func (ctx *RestrictedContext) CreateSecret(name string, args *UpsertArgs) (string, error) {
	return "", ErrRestrictedContext
}
