package constants

const (
	HTTP_CLIENT_TIMEOUT_SECONDS = 30

	CYCLE_FETCH_FREQUENCY_MINUTES = 5
	MINIMUM_DIFF_TOLERANCE        = 1

	RPC_INIT_BATCH_SIZE       = 3
	DELEGATE_FETCH_BATCH_SIZE = 8
	CONTRACT_FETCH_BATCH_SIZE = 50

	BALANCE_FETCH_RETRY_DELAY_SECONDS = 20
	BALANCE_FETCH_RETRY_ATTEMPTS      = 3

	LOG_LEVEL              = "LOG_LEVEL"
	LISTEN                 = "LISTEN"
	LISTEN_DEFAULT         = "127.0.0.1:3000"
	PRIVATE_LISTEN         = "PRIVATE_LISTEN"
	PRIVATE_LISTEN_DEFAULT = ""

	STORED_CYCLES = 20
)

type StorageKind string

const (
	Archive StorageKind = "archive"
	Rolling StorageKind = "rolling"
)
