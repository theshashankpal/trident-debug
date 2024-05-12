/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/theshashankpal/trident_debug/debug"
)

const (
	copyDirectory = "copy_directory"
)

var (
	artifactoryNamespace string
	artifactoryFolder    string
	kubeConfigPath       string
)

func init() {
	RootCmd.PersistentFlags().StringVarP(&artifactoryNamespace, "artifactory", "a", "",
		"Input the namespace of your artifactory for example: docker.eng.netapp.com/pshashan")
	RootCmd.PersistentFlags().StringVarP(&artifactoryFolder, "folder", "f", "",
		"folder in which image will be pushed for example: docker.eng.netapp.com./pshashan/trident-debug")
	RootCmd.PersistentFlags().StringVarP(&kubeConfigPath, "kubeconfig", "k", "", "Kubernetes config path")
	RootCmd.SetOut(os.Stdout)
}

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:          "trident-debug",
	Short:        "Starts a remote debugger for trident",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {

		if artifactoryNamespace == "" {
			return fmt.Errorf("artifactory namespace is required. Please provide the namespace using --artifactory or -a flag")
		}

		err := os.MkdirAll("./"+copyDirectory, 0755)
		if err != nil {
			fmt.Printf("Cannot create copy_directory: %v\n", err)
			return err
		}

		// Copying the Makefile to the copy directory
		cmdCopy := exec.Command("cp", "./../Makefile", copyDirectory+"/")
		err = cmdCopy.Run()
		if err != nil {
			fmt.Println("Cannot copy makefile to copy_directory: ", err)
			return err
		}

		// Copying the Dockerfile to the copy directory
		cmdCopy = exec.Command("cp", "./../Dockerfile", copyDirectory+"/")
		err = cmdCopy.Run()
		if err != nil {
			fmt.Println("Cannot copy dockerfile to copy_directory: ", err)
			return err
		}

		// Copying the Makefile to the copy directory
		cmdCopy = exec.Command("cp", "./Makefile", "./..")
		err = cmdCopy.Run()
		if err != nil {
			fmt.Println("Cannot copy makefile to parent directory: ", err)
			return err
		}

		// Copying the Dockerfile to the copy directory
		cmdCopy = exec.Command("cp", "./Dockerfile", "./..")
		err = cmdCopy.Run()
		if err != nil {
			fmt.Println("Cannot copy dockerfile to parent directory:", err)
			return err
		}

		cmdMake := exec.Command("make", "debug", "ARTIFACTORY_NAMESPACE="+artifactoryNamespace, "ARTIFACTORY_FOLDER="+artifactoryFolder, "-C", "./..")
		cmdMake.Stdout = os.Stdout
		cmdMake.Stderr = os.Stderr
		err = cmdMake.Run()
		if err != nil {
			fmt.Println("Cannot run make debug: ", err)
			return err
		}

		// Creating a context with cancel. This will be used to stop the process
		ctx, cancel := context.WithCancel(context.Background())

		// Error channel to capture any errors during the debug process.
		errChan := make(chan error, 1)
		// used for signaling whether the trident deployment.yaml in kubernetes is successful or not.
		deploymentChan := make(chan int, 1)
		// Adding the deploymentChan to the context.
		ctx = context.WithValue(ctx, "deploymentChan", deploymentChan)

		// Wait group to wait for the debug process to complete.
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := debug.InitDebug(ctx, kubeConfigPath, artifactoryNamespace, artifactoryFolder); err != nil {
				errChan <- err
			} else {
				errChan <- nil
			}
		}()

		fmt.Println("Waiting for the deployment to be successful ")
		<-deploymentChan // Wait for the deployment to be successful

		fmt.Println("Press 'exit' to stop the process...")
		go func() {
			scanner := bufio.NewScanner(os.Stdin)
			for scanner.Scan() {
				input := scanner.Text()
				if strings.ToLower(input) == "exit" {
					cancel()
					return
				}
			}
		}()

		go func() {
			sigint := make(chan os.Signal, 1)
			signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
			<-sigint
			cancel()
		}()

		wg.Wait()
		fmt.Println("Stopping the process...")

		cmdCopy = exec.Command("cp", copyDirectory+"/"+"Makefile", "./..")
		err = cmdCopy.Run()
		if err != nil {
			return err
		}

		cmdCopy = exec.Command("cp", copyDirectory+"/"+"Dockerfile", "./..")
		err = cmdCopy.Run()
		if err != nil {
			return err
		}

		select {
		case err := <-errChan:
			if err != nil {
				return err
			} else {
				return nil
			}
		}
	},
}
