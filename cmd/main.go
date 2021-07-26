package main

import (
	"fmt"
	"github.com/spf13/cobra"
)

func Main() {
	var newCmd = &cobra.Command{
		Use:   "new",
		Short: "A brief description of your command",
		Long:  `A longer description...`,
		Run: func(cmd *cobra.Command, args []string) {

		},
	}

	newCmd.Flags().Int("intf", 0, "Set Int")
	newCmd.Flags().String("stringf", "sss", "Set String")
	newCmd.Flags().Bool("q", false, "Set Bool")

	newCmd.Flags().IntP("aaa", "a", 1, "Set A")
	newCmd.Flags().IntP("bbb", "b", -1, "Set B")

	root := cobra.Command{
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(args, cmd.Flag("daemon").Value)
		},
	}
	root.Flags().BoolP("daemon", "d", false, "daemon")

	root.AddCommand(newCmd)

	root.Execute()
}

func main() {
	Main()
}
