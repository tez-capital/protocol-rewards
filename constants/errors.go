package constants

import "errors"

var (
	ErrNotFound          = errors.New("not found")
	ErrCycleDidNotEndYet = errors.New("cycle did not end yet")

	ErrDelegateHasNoMinimumDelegatedBalance = errors.New("delegate has no minimum delegated balance")

	ErrFailedToFetchContract              = errors.New("failed to fetch contract")
	ErrBalanceNotFoundInDelegationState   = errors.New("balance not found in delegation state")
	ErrDelegatorNotFoundInDelegationState = errors.New("delegator not found in delegation state")
	ErrMinimumDelegatedBalanceNotFound    = errors.New("minimum delegated balance not found")
	ErrFailedToFetchContractBalances      = errors.New("failed to fetch contract balances")
)
