package aliyun

import (
	"errors"
	"log"
	"path/filepath"

	"github.com/tiechui1994/tool/util"
)

func GetCacheData() (roles []Role, org Org, spaces []Space, err error) {
	var result struct {
		Roles  []Role
		Org    Org
		Spaces []Space
	}

	key := filepath.Join(util.ConfDir, "teambition_cache.json")
	if util.ReadFile(key, &result) == nil {
		return result.Roles, result.Org, result.Spaces, nil
	}

	roles, err = Roles()
	if err != nil {
		log.Println(err)
		return
	}

	if len(roles) == 0 {
		err = errors.New("no roles")
		return
	}

	org, err = Orgs(roles[0].OrganizationId)
	if err != nil {
		log.Println(err)
		return
	}

	var user User
	user, err = GetByUser(roles[0].OrganizationId)
	if err != nil {
		log.Println(err)
		return
	}

	spaces, err = Spaces(user.OrganizationId, user.ID)
	if err != nil {
		log.Println(err)
		return
	}

	if len(spaces) == 0 {
		err = errors.New("no spaces")
		return
	}

	result.Roles = roles
	result.Org = org
	result.Spaces = spaces
	util.WriteFile(key, result)

	return
}
