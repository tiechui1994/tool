package zerotier

import (
	"encoding/json"
	"github.com/tiechui1994/tool/util"
)

type Zerotier struct {
	Token string
}

const (
	api = "https://my.zerotier.com"
)

type IPRange struct {
	IPRangeStart string `json:"ipRangeStart"`
	IPRangeEnd   string `json:"ipRangeEnd"`
}

type Route struct {
	Target string `json:"target"`
	Via    string `json:"via"`
}

type Network struct {
	ID          string `json:"id"`
	RulesSource string `json:"rulesSource"`
	Config      struct {
		Dns struct {
			Domain  string   `json:"domain"`
			Servers []string `json:"servers"`
		} `json:"dns"`
		EnableBroadcast   bool      `json:"enableBroadcast"`
		Mtu               int       `json:"mtu"`
		IpAssignmentPools []IPRange `json:"ipAssignmentPools"`
		MulticastLimit    int       `json:"multicastLimit"`
		Name              string    `json:"name"`
		Private           bool      `json:"private"`
		Routes            []Route   `json:"routes"`
	} `json:"config"`
}

func (z *Zerotier) GetNetworks() (netwoks []Network, err error) {
	u := api + "/api/v1/network"
	header := map[string]string{
		"content-type":  "application/json",
		"authorization": "bearer " + z.Token,
	}

	raw, err := util.GET(u, header)
	if err != nil {
		return netwoks, err
	}

	err = json.Unmarshal(raw, &netwoks)
	return netwoks, err
}

type Member struct {
	ID              string `json:"id"`
	NetworkId       string `json:"networkId"`
	NodeId          string `json:"nodeId"`
	Name            string `json:"name"`
	PhysicalAddress string `json:"physicalAddress"`
	ClientVersion   string `json:"clientVersion"`
	Hidden          bool   `json:"hidden"`
	Config          struct {
		ActiveBridge    bool     `json:"activeBridge"`
		Authorized      bool     `json:"authorized"`
		Capabilities    []int    `json:"capabilities"`
		IpAssignments   []string `json:"ipAssignments"`
		NoAutoAssignIps bool     `json:"noAutoAssignIps"`
		Tags            []int    `json:"tags"`
	} `json:"config"`
}

func (z *Zerotier) GetMembers(networkid string) (members []Member, err error) {
	u := api + "/api/v1/network/" + networkid + "/member"
	header := map[string]string{
		"content-type":  "application/json",
		"authorization": "bearer " + z.Token,
	}

	raw, err := util.GET(u, header)
	if err != nil {
		return members, err
	}

	err = json.Unmarshal(raw, &members)
	return members, err
}
func (z *Zerotier) UpdateMember(networkid string, m Member) (err error) {
	u := api + "/api/v1/network/" + networkid + "/member/" + m.ID
	header := map[string]string{
		"content-type":  "application/json",
		"authorization": "bearer " + z.Token,
	}

	body := map[string]interface{}{
		"hidden": m.Hidden,
		"name":   m.Name,
		"config": m.Config,
	}

	_, err = util.POST(u, body, header)
	return err
}
func (z *Zerotier) DeleteMember(networkid, memberid string) (err error) {
	u := api + "/api/v1/network/" + networkid + "/member/" + memberid
	header := map[string]string{
		"content-type":  "application/json",
		"authorization": "bearer " + z.Token,
	}

	_, err = util.DELETE(u, header)
	return err
}
