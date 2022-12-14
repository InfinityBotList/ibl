/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/InfinityBotList/ibl/helpers"
	"github.com/infinitybotlist/eureka/crypto"
	"github.com/spf13/cobra"
)

func mkBackup() {
	cleanup := func() {
		fmt.Println("Cleaning up...")

		// delete all files in work directory
		err := os.RemoveAll("work")

		if err != nil {
			fmt.Println("Error cleaning up:", err)
		}
	}

	cleanup()

	// create a work directory
	err := os.Mkdir("work", 0755)

	if err != nil {
		fmt.Println("Error creating work directory:", err)
		cleanup()
		return
	}

	var passFile = "/certs/bakkey"

	if os.Getenv("ALT_BAK_KEY") != "" {
		passFile = os.Getenv("ALT_BAK_KEY")
	}

	var outFolder = "/certs/backups"

	if os.Getenv("ALT_BAK_OUT") != "" {
		outFolder = os.Getenv("ALT_BAK_OUT")
	}

	// Read the password from the file
	pass, err := os.ReadFile(passFile)

	if err != nil {
		cleanup()
		fmt.Println(err)
		return
	}

	// Encrypt sample data
	salt := crypto.RandString(8)
	passHashed := helpers.GenKey(string(pass), salt)

	// Create a new backup using pg_dump
	backupCmd := exec.Command("pg_dump", "-Fc", "--no-owner", "-d", "infinity", "-f", "work/schema.sql")

	backupCmd.Env = helpers.GetEnv()

	err = backupCmd.Run()

	if err != nil {
		fmt.Println(err)
		cleanup()
		return
	}

	file, err := os.ReadFile("work/schema.sql")

	if err != nil {
		fmt.Println(err)
		cleanup()
		return
	}

	// Cleanup work now
	cleanup()

	// Encrypt the file
	encFile, err := helpers.Encrypt(passHashed, file)

	if err != nil {
		fmt.Println(err)
		cleanup()
		return
	}

	// Get current datetime formatted
	t := time.Now().Format("2006-01-02-15:04:05")

	// Write the encrypted file
	err = os.WriteFile(outFolder+"/backup-"+t+".iblbackup", encFile, 0604)

	if err != nil {
		fmt.Println(err)
		cleanup()
		return
	}
}

// backupdbCmd represents the backupdb command
var backupdbCmd = &cobra.Command{
	Use:   "backupdb",
	Short: "Database backup commands",
	Long:  `Database backup commands`,
}

var backupdbNewCmd = &cobra.Command{
	Use:   "new",
	Short: "Create a new database backup",
	Long:  `Create a new database backup`,
	Run: func(cmd *cobra.Command, args []string) {
		mkBackup()
	},
}

var longRunningBackupCmd = &cobra.Command{
	Use:   "long",
	Short: "Create a new database backup every 8 hours",
	Long:  `Create a new database backup every 8 hours`,
	Run: func(cmd *cobra.Command, args []string) {
		for {
			fmt.Println("Making backup at", time.Now())
			mkBackup()
			fmt.Println("Sleeping for 8 hours...")
			time.Sleep(8 * time.Hour)
		}
	},
}

func init() {
	backupdbCmd.AddCommand(backupdbNewCmd)
	backupdbCmd.AddCommand(longRunningBackupCmd)
	rootCmd.AddCommand(backupdbCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// backupdbCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// backupdbCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
