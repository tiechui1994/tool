package aliyun

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"text/template"

	"github.com/tiechui1994/tool/aliyun/teambition"
)

func TestList(t *testing.T) {
	tpl := `
total {{(len .)}}
{{- range . }}
{{ $x := "f" }}
{{- if (eq .Type 1) -}} 
	{{- $x = "d" -}}
{{- end -}}
{{printf "%s  %24s  %s" $x .Updated .Name}}
{{- end }}
`
	tpl = strings.Trim(tpl, "\n")
	temp, err := template.New("").Parse(tpl)
	if err != nil {
		fmt.Println(err)
		return
	}

	list := []FileNode{
		{
			Name:    "111",
			Updated: "2021-04-24T07:36:48.591Z",
			Type:    1,
		},
		{
			Name:    "222",
			Updated: "2021-04-24T07:36:48.591Z",
			Type:    2,
		},
		{
			Name:    "333333",
			Updated: "2021-04-24T07:36:48.591Z",
			Type:    2,
		},
	}

	var buf bytes.Buffer
	err = temp.Execute(&buf, list)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(buf.String())
}

func TestOrgs(t *testing.T) {
	tpl := `
{{ range $orgidx, $ele := . }}
{{ printf "%d. org: %s(%s)" $orgidx  .Name .OrganizationId -}}
{{ range $pidx, $val := .Projects }}
  {{ printf "%d.%d project: %s(%s)" $orgidx $pidx .Name .ProjectId -}}
{{ end }}
{{ end }}
`
	tpl = strings.Trim(tpl, "\n")
	temp, err := template.New("").Parse(tpl)
	if err != nil {
		fmt.Println(err)
		return
	}

	orgs := []teambition.Org{
		{
			OrganizationId: "111111",
			Name:           "java",
			Projects: []teambition.Project{
				{
					ProjectId: "122122112",
					Name:      "j112212ava-1",
				},
				{
					ProjectId: "122122112",
					Name:      "java-11",
				},
			},
		},
		{
			OrganizationId: "222",
			Name:           "c++",
			Projects: []teambition.Project{
				{
					ProjectId: "2222222222",
					Name:      "c++-99",
				},
				{
					ProjectId: "2222222222",
					Name:      "c++-11",
				},
			},
		},
	}
	var buf bytes.Buffer
	err = temp.Execute(&buf, orgs)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(buf.String())
}

func TestAlign(t *testing.T) {
	x := uint(54)
	y := uint(16)

	fmt.Println(x + (y - x&^y))
}
