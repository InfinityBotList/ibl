/*
Copyright © 2022 Infinity Bot List
*/
package cmd

import (
	"fmt"
	"os"
	"runtime/debug"
	"time"

	"github.com/spf13/cobra"
)

var (
	// Store git commit hash
	GitCommit string
	// This is gotten through ldflags
	BuildRev string
	// Build time is the time the binary was built
	BuildTime string
)

func init() {
	// Use runtime/debug vcs.revision to get the git commit hash
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				GitCommit = setting.Value
			}
		}
	}

	if GitCommit == "" {
		GitCommit = "unknown"
	}
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "ibl",
	Short: "IBL is a simple utility to make development of IBL easier as well as to allow bot developers to test their bots.",
	Long: `IBL is a simple utility to make development of Infinity Bot List easier as well as to allow bot developers to test the API. 

For more information, try running "ibl --help"`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Legacy update code
	if os.Getenv("IN_UPDATE") == "1" {
		fmt.Println("Update successful!\n\nversion:", GitCommit, "| pcPath:", os.Getenv("PC_PATH"), "| buildRev:", BuildRev, "| buildTime:", BuildTime)

		// Give time for old process to exit
		time.Sleep(500 * time.Millisecond)

		// Rename new binary
		os.Rename(os.Getenv("PC_PATH")+".new", os.Getenv("PC_PATH"))
		os.Rename(os.Getenv("PC_PATH")+".new.exe", os.Getenv("PC_PATH"))
		os.Exit(0)
	}

	// Here you will define your flags and configuration settings.
	// Cobra supports persstent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.ibl.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
