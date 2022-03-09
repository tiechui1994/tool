package main

import "github.com/tiechui1994/tool/zerotier"

func main() {
	zero  := zerotier.Zerotier{Token:""}
	zero.GetMembers("")
}