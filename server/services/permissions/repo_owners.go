package permissions

import (
	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/shared/utils"
)

func NewOwnersAllowlist(owners []string) *OwnersAllowlist {
	return &OwnersAllowlist{owners: utils.SliceToBoolMap(owners)}
}

type OwnersAllowlist struct {
	owners map[string]bool
}

func (o *OwnersAllowlist) IsAllowed(repo *model.Repo) bool {
	return len(o.owners) < 1 || o.owners[repo.Owner]
}
