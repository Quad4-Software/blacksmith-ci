package org

import (
	"context"
	"os"
	"strings"
	"text/template"

	"github.com/urfave/cli/v3"

	"codeberg.org/crowci/crow/v5/cli/common"
	"codeberg.org/crowci/crow/v5/cli/internal"
	"codeberg.org/crowci/crow/v5/crow-go/crow"
)

var Command = &cli.Command{
	Name:  "org",
	Usage: "manage organizations",
	Commands: []*cli.Command{
		orgListCmd,
	},
}

var orgListCmd = &cli.Command{
	Name:      "ls",
	Usage:     "list organizations",
	ArgsUsage: "",
	Action:    orgList,
	Flags: []cli.Flag{
		common.FormatFlag(tmplOrgList, true),
	},
}

func orgList(ctx context.Context, c *cli.Command) error {
	format := c.String("format") + "\n"

	client, err := internal.NewClient(ctx, c)
	if err != nil {
		return err
	}

	opt := crow.ListOptions{}

	list, err := client.OrgList(opt)
	if err != nil {
		return err
	}

	tmpl, err := template.New("_").Funcs(orgFuncMap).Parse(format)
	if err != nil {
		return err
	}

	for _, org := range list {
		if err := tmpl.Execute(os.Stdout, org); err != nil {
			return err
		}
	}
	return nil
}

// Template for org list items.
var tmplOrgList = "\x1b[33m{{ .Name }} \x1b[0m" + `
Organization ID: {{ .ID }}
`

var orgFuncMap = template.FuncMap{
	"list": func(s []string) string {
		return strings.Join(s, ", ")
	},
}
