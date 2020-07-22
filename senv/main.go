package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/jamowei/senv"
	"github.com/spf13/cobra"
)

const hostDefault, portDefault, nameDefault, labelDefault, tokenDefault = "127.0.0.1", "8080", "application", "master", ""

var profileDefault = []string{"default"}

var version = "1.0.0"
var date = "2020"

var (
	host, port, name, label, token    string
	profiles                          []string
	noSysEnv, json, verbose, sanitize bool
)

var errExitCode = 0

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	} else if errExitCode > 0 {
		os.Exit(errExitCode)
	}
}

var rootCmd = &cobra.Command{
	Use:     "senv [command]",
	Short:   "A native config-client for the spring-cloud-config-server",
	Version: "v" + version,
	Long: fmt.Sprintf(
		`v%s                             © %s Jan Weidenhaupt

Senv is a fast native config-client for a
spring-cloud-config-server written in Go`, version, date[:4]),
	Args: cobra.NoArgs,
}

func warningDefault(_ *cobra.Command, _ []string) {
	if name == nameDefault {
		fmt.Fprintln(os.Stderr, "warning: no application name given, using default 'application'")
	}
}

var envCmd = &cobra.Command{
	Use:   "env [command]",
	Short: "Fetches properties and sets them as environment variables",
	Long: `Fetches properties from the spring-cloud-config-server
and replaces the placeholder in the specified command.`,
	Example: `on config-server:
  spring.application.name="Senv"
Example call:
  senv env echo ${spring.application.name:default}  //prints 'Senv' or when config-server not reachable 'default'`,
	PreRun:       warningDefault,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := senv.NewConfig(host, port, name, profiles, label, token)
		if err := cfg.Fetch(json, verbose); err != nil {
			return err
		}
		if err := cfg.Process(); err != nil {
			return err
		}
		return runCommand(args, cfg.Properties, noSysEnv)
	},
}

func runCommand(args []string, props map[string]string, noSysEnv bool) error {
	cmd := exec.Command(args[0], args[1:]...)
	if !noSysEnv {
		cmd.Env = os.Environ()
	}
	for key, element := range props {
		var envVarKey = key
		if sanitize {
			envVarKey = strings.ReplaceAll(key, ".", "_")
			envVarKey = strings.ReplaceAll(key, "-", "_")
			envVarKey = strings.ToUpper(envVarKey)
		}
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", envVarKey, element))
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = io.MultiWriter(os.Stdout, &stdoutBuf)
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderrBuf)
	err := cmd.Run()

	if err != nil {
		// try to get the exit code
		if exitError, ok := err.(*exec.ExitError); ok {
			ws := exitError.Sys().(syscall.WaitStatus)
			errExitCode = ws.ExitStatus()
		}
	} else {
		// success, errExitCode should be 0 if go is ok
		ws := cmd.ProcessState.Sys().(syscall.WaitStatus)
		errExitCode = ws.ExitStatus()
	}

	outStr, errStr := string(stdoutBuf.Bytes()), string(stderrBuf.Bytes())
	fmt.Printf("\nout:\n%s\nerr:\n%s\n", outStr, errStr)

	return nil
}

func init() {
	envCmd.PersistentFlags().BoolVarP(&noSysEnv, "nosysenv", "s", false, "start without system-environment variables")
	envCmd.PersistentFlags().BoolVarP(&json, "json", "j", false, "print json to stdout")
	envCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose")
	envCmd.PersistentFlags().BoolVarP(&sanitize, "sanitize", "x", true, "sanitize keys")
	rootCmd.PersistentFlags().StringVar(&host, "host", hostDefault, "configserver host")
	rootCmd.PersistentFlags().StringVar(&port, "port", portDefault, "configserver port")
	rootCmd.PersistentFlags().StringVarP(&name, "name", "n", nameDefault, "spring.application.name")
	rootCmd.PersistentFlags().StringVarP(&token, "token", "t", tokenDefault, "x-config-token")
	rootCmd.PersistentFlags().StringSliceVarP(&profiles, "profiles", "p", profileDefault, "spring.active.profiles")
	rootCmd.PersistentFlags().StringVarP(&label, "label", "l", labelDefault, "config-repo label to be used")
	rootCmd.AddCommand(envCmd)
}
