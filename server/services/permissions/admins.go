package permissions

import (
	"codeberg.org/crowci/crow/v5/server/model"
	"codeberg.org/crowci/crow/v5/shared/utils"
)

func NewAdmins(admins []string) *Admins {
	return &Admins{admins: utils.SliceToBoolMap(admins)}
}

type Admins struct {
	admins map[string]bool
}

func (a *Admins) IsAdmin(user *model.User) bool {
	return a.admins[user.Login]
}
