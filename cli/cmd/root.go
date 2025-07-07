package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// banner is ASCII art of a brain in bold blue
var banner = "\033[1;34m\n" +
	"      _---~~(~~-_.\n" +
	"    _{        )   )\n" +
	"  ,   ) -~~- ( ,-' )_\n" +
	" (  `-,_..`., )-- '_,)\n" +
	"( ` _)  (  -~( -_ `,  }\n" +
	"(_-  _  ~_-~~~~`,  ,' )\n" +
	"  `~ -^(    __;-,((()))\n" +
	"        ~~~~ {_ -_(())\n" +
	"               `\\  }\n" +
	"                 { }\n" +
	"\033[0m"

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "mind [flags]",
	Short: "command line inteface for cypher-mind",
	Long:  banner + "\nA Command-line interface for CypherMind AI Assistant",
}

// Execute runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
