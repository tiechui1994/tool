package main

import "github.com/tiechui1994/tool/zerotier"

func main() {
	zero  := zerotier.Zerotier{Token:"wkDIdDehQOdtcurgkNhDZQfZjeLsGkVh"}
	zero.GetMembers("159924d63015b18d")
}