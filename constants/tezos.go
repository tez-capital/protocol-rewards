package constants

import "slices"

type IgnoredBalanceUpdateKindsType []string

func (k IgnoredBalanceUpdateKindsType) Contains(kind string) bool {
	return slices.Contains(k, kind)
}

var (
	IgnoredBalanceUpdateKinds = IgnoredBalanceUpdateKindsType{
		"burned",
	}
)
