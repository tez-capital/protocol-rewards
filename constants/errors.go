package constants

import "errors"

var (
	ErrSubsystemNotFound = errors.New("subsystem not found")

	ErrDelegateHasNoMinimumDelegatedBalance = errors.New("delegate has no minimum delegated balance")

	ErrFailedToFetchContract              = errors.New("failed to fetch contract")
	ErrBalanceNotFoundInDelegationState   = errors.New("balance not found in delegation state")
	ErrDelegatorNotFoundInDelegationState = errors.New("delegator not found in delegation state")
)
