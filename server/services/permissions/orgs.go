package permissions

import (
	"codefloe.com/crowci/crow/v5/server/model"
	"codefloe.com/crowci/crow/v5/shared/utils"
)

func NewOrgs(orgs []string) *Orgs {
	return &Orgs{
		IsConfigured: len(orgs) > 0,
		orgs:         utils.SliceToBoolMap(orgs),
	}
}

type Orgs struct {
	IsConfigured bool
	orgs         map[string]bool
}

func (o *Orgs) IsMember(teams []*model.Team) bool {
	for _, team := range teams {
		if o.orgs[team.Login] {
			return true
		}
	}
	return false
}
