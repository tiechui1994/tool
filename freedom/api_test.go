package freedom

import (
	"fmt"
	"testing"
)

func TestFreedom_Login(t *testing.T) {
	fr := New("", "")
	//fr.needAccessToken()
	fmt.Println(fr.Login())
	fmt.Println(fr.Domain())
}
