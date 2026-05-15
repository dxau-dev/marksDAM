package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	_ "github.com/mattn/go-sqlite3"

	"github.com/dxau-dev/marksDAM/cmd"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "setup":
		setupCmd := flag.NewFlagSet("setup", flag.ContinueOnError)
		configPath := setupCmd.String("path", "", "Relative path for config and database directory (required)")
		if err := setupCmd.Parse(os.Args[2:]); err != nil {
			log.Fatal(err)
		}
		if *configPath == "" {
			fmt.Println("Error: -path is required for setup")
			setupCmd.Usage()
			os.Exit(1)
		}
		if err := cmd.Setup(*configPath); err != nil {
			log.Fatal(err)
		}

	case "submit":
		submitCmd := flag.NewFlagSet("submit", flag.ContinueOnError)
		configPath := submitCmd.String("config", "", "Path to config and database directory (required)")
		if err := submitCmd.Parse(os.Args[2:]); err != nil {
			log.Fatal(err)
		}
		if *configPath == "" {
			fmt.Println("Error: -config is required for submit")
			submitCmd.Usage()
			os.Exit(1)
		}
		if err := cmd.Submit(*configPath); err != nil {
			log.Fatal(err)
		}

	case "retrieve":
		retrieveCmd := flag.NewFlagSet("retrieve", flag.ContinueOnError)
		configPath := retrieveCmd.String("config", "", "Path to config and database directory (required)")
		if err := retrieveCmd.Parse(os.Args[2:]); err != nil {
			log.Fatal(err)
		}
		if *configPath == "" {
			fmt.Println("Error: -config is required for retrieve")
			retrieveCmd.Usage()
			os.Exit(1)
		}
		if err := cmd.Retrieve(*configPath); err != nil {
			log.Fatal(err)
		}

	case "config":
		configCmd := flag.NewFlagSet("config", flag.ContinueOnError)
		configPath := configCmd.String("path", "", "Path to config and database directory (required)")
		printFlag := configCmd.Bool("print", false, "Print current configuration")
		if err := configCmd.Parse(os.Args[2:]); err != nil {
			log.Fatal(err)
		}
		if *configPath == "" {
			fmt.Println("Error: -path is required for config")
			configCmd.Usage()
			os.Exit(1)
		}
		if err := cmd.ShowConfig(*configPath, *printFlag); err != nil {
			log.Fatal(err)
		}

	case "help", "-h", "--help":
		printUsage()

	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`Mark's DAM - Digital Asset Management CLI

Usage:
  marksdam <command> [options]

Commands:
  setup     Initialize config and database in a directory
            -path <dir>    Relative path for config/database directory (required)

  submit    Scan directory for images and submit batch to OpenAI
            -config <dir>  Path to config/database directory (required)

  retrieve  Poll OpenAI for batch results and update database
            -config <dir>  Path to config/database directory (required)

  config    Show or validate configuration
            -path <dir>    Path to config/database directory (required)
            -print         Print current configuration

  help      Show this help message

Examples:
  marksdam setup -path ./dam-config
  marksdam submit -config ./dam-config
  marksdam retrieve -config ./dam-config
  marksdam config -path ./dam-config -print`)
}
