package aliyun

import (
	"errors"
	"log"
	"path/filepath"

	"github.com/tiechui1994/tool/util"
)

func GetCacheData() (roles []Role, org Org, err error) {
	var result struct {
		Roles []Role
		Org   Org
	}

	key := filepath.Join(util.ConfDir, "teambition_cache.json")
	if util.ReadFile(key, &result) == nil {
		return result.Roles, result.Org, nil
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

	result.Roles = roles
	result.Org = org
	util.WriteFile(key, result)
	return
}
