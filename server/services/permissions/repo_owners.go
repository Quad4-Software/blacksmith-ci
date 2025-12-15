package permissions

import (
	"codeberg.org/crowci/crow/v5/server/model"
	"codeberg.org/crowci/crow/v5/shared/utils"
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
