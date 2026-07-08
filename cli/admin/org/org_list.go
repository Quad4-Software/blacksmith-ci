package org

import (
	"context"
	"fmt"
	"io"
	"os"
	"text/template"

	"github.com/urfave/cli/v3"

	"codefloe.com/crowci/crow/v6/cli/common"
	"codefloe.com/crowci/crow/v6/cli/internal"
	"codefloe.com/crowci/crow/v6/cli/output"
	"codefloe.com/crowci/crow/v6/crow-go/crow"
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
	Flags:     common.OutputFlags("table"),
}

func orgList(ctx context.Context, c *cli.Command) error {
	client, err := internal.NewClient(ctx, c)
	if err != nil {
		return err
	}

	opt := crow.ListOptions{}

	list, err := client.OrgList(opt)
	if err != nil {
		return err
	}

	return orgOutput(c, list)
}

func orgOutput(c *cli.Command, orgs []*crow.Org, fd ...io.Writer) error {
	outFmt, outOpt := output.ParseOutputOptions(c.String("output"))
	noHeader := c.Bool("output-no-headers")

	var out io.Writer = os.Stdout
	if len(fd) > 0 {
		out = fd[0]
	}

	switch outFmt {
	case "json":
		return output.WriteJSON(out, orgs)
	case "yaml":
		return output.WriteYAML(out, orgs)
	case "go-template":
		if len(outOpt) < 1 {
			return fmt.Errorf("%w: missing template", output.ErrOutputOptionRequired)
		}
		tmpl, err := template.New("_").Parse(outOpt[0] + "\n")
		if err != nil {
			return err
		}
		return tmpl.Execute(out, orgs)
	case "table":
		fallthrough
	default:
		table := output.NewTable(out)
		cols := []string{"ID", "Name", "Is_User"}
		if len(outOpt) > 0 {
			cols = outOpt
		}
		if !noHeader {
			table.WriteHeader(cols)
		}
		for _, org := range orgs {
			if err := table.Write(cols, org); err != nil {
				return err
			}
		}
		table.Flush()
	}

	return nil
}
