package permissions

import (
	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/shared/utils"
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
