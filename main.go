package main

import (
	"github.com/ahdekkers/gofs/gofs"
	"github.com/spf13/cobra"
	"log"
)

func main() {
	opts := gofs.Opts{}
	rootCmd := &cobra.Command{
		Use:   "gofs",
		Short: "gofs -p 9092 -addr localhost -rootDir /home/me/serverRoot/ -level DEBUG -logFile /home/me/serverLog.txt",
		Long:  "gofs is a simple http file server, written in Go.",
		Run: func(cmd *cobra.Command, args []string) {
			err := gofs.Create(opts)
			if err != nil {
				cmd.Printf("Error while running gofs: %v\n", err)
			}
		},
	}

	flags := rootCmd.Flags()
	flags.StringVarP(&opts.Addr, "addr", "a", "localhost",
		"The address to bind to, excluding the port number")
	flags.IntVarP(&opts.Port, "port", "p", 9092,
		"The port which the http server will be available on")
	flags.StringVarP(&opts.RootDir, "rootDir", "r", "",
		"The root dir for storing files. Any given paths will be relative to this directory")
	flags.StringVar(&opts.LogLevel, "level", "DEBUG",
		"The level for log output")
	flags.StringVar(&opts.LogFile, "logFile", "",
		"A file to write log output to, as well as stdOut")

	err := rootCmd.MarkFlagRequired("rootDir")
	if err != nil {
		log.Printf("[WARN]  Failed to mark rootDir flag as required: %v\n", err)
	}
	err = rootCmd.MarkFlagDirname("rootDir")
	if err != nil {
		log.Printf("[WARN]  Failed to mark rootDir flag as dirname: %v\n", err)
	}
	err = rootCmd.MarkFlagRequired("logFile")
	if err != nil {
		log.Printf("[WARN]  Failed to mark logFile flag as required: %v\n", err)
	}
	err = rootCmd.MarkFlagFilename("logFile", ".txt")
	if err != nil {
		log.Printf("[WARN]  Failed to mark logFile flag as filename: %v\n", err)
	}

	err = rootCmd.Execute()
	if err != nil {
		log.Printf("[ERROR] Failed to execute main command: %v\n", err)
	}
}
