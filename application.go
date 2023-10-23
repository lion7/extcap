package extcap

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/urfave/cli/v2"
)

// App is the main structure of an extcap application.
type App struct {
	// Application brief description
	Usage string

	// Application help description
	HelpPage string

	// AdditionalHelp will be displayed in version output.
	Version VersionInfo

	// Usage examples to display in USAGE section of help output.
	// Every string will be prepended with <application-name>
	// By default, the following usage example is always added:
	//    --extcap-interfaces
	//
	// As an example, adding the following lines with application name 'ciscodump':
	//    --extcap-interface=ciscodump --extcap-dlts
	//    --extcap-interface=ciscodump --extcap-config
	//    --extcap-interface=ciscodump --remote-host myhost --remote-port 22222 --remote-username myuser --remote-interface gigabit0/0 --fifo=FILENAME --capture
	// will produce the following output:
	//    ciscodump --extcap-interfaces
	//    ciscodump --extcap-interface=ciscodump --extcap-dlts
	//    ciscodump --extcap-interface=ciscodump --extcap-config
	//    ciscodump --extcap-interface=ciscodump --remote-host myhost --remote-port 22222 --remote-username myuser --remote-interface gigabit0/0 --fifo=FILENAME --capture
	UsageExamples []string

	// GetInterfaces returns list of interfaces. Should be implemented.
	GetInterfaces func() ([]CaptureInterface, error)

	// GetDLT returns DLT for given interface. Should be implemented.
	GetDLT func(iface string) (DLT, error)

	// GetConfigOptions returns configuration parameters for given interface. Optional.
	GetConfigOptions func(iface string) ([]ConfigOption, error)

	// GetAllConfigOptions returns all possible configuration options. Optional (interfaces do not have any configuration options).
	GetAllConfigOptions func() []ConfigOption

	// StartCapture starts capture process. Should be implemented. Opts are the configuration options for capture on given interface.
	StartCapture func(iface string, fifo io.WriteCloser, filter string, opts map[string]interface{}) error

	// OpenPipe opens fifo pipe to write capture results. If it is not defined then default is used.
	OpenPipe func(string) (io.WriteCloser, error)
}

// Run executes the main application loop
func (extapp App) Run(arguments []string) {
	app := cli.NewApp()

	// set version information
	if extapp.Version.Info == "" {
		extapp.Version.Info = "0.0.1"
	}
	if extapp.Version.Help == "" {
		extapp.Version.Help = "https://github.com/lion7/extcap"
	}

	app.Usage = extapp.Usage
	app.Description = extapp.HelpPage

	// generate usage examples
	extapp.UsageExamples = append([]string{"--extcap-interfaces"}, extapp.UsageExamples...)
	w := new(strings.Builder)
	for i, str := range extapp.UsageExamples {
		_, _ = fmt.Fprintf(w, "%s %s", app.Name, str)
		if i != len(extapp.UsageExamples)-1 {
			_, _ = fmt.Fprintln(w)
		}
	}
	app.UsageText = w.String()

	app.CustomAppHelpTemplate = helpTemplate

	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:  "extcap-version",
			Usage: "specify the Wireshark major and minor version",
		},

		&cli.BoolFlag{
			Name:  "extcap-interfaces",
			Usage: "list the extcap interfaces",
		},

		&cli.BoolFlag{
			Name:  "extcap-dlts",
			Usage: "list the DLTs",
		},

		&cli.StringFlag{
			Name:  "extcap-interface",
			Usage: "specify the extcap interface `<iface>`",
		},

		&cli.BoolFlag{
			Name:  "extcap-config",
			Usage: "list the additional configuration for an interface",
		},

		&cli.BoolFlag{
			Name:  "capture",
			Usage: "run the capture",
		},

		&cli.StringFlag{
			Name:  "extcap-capture-filter",
			Usage: "the capture filter `<filter>`",
		},

		&cli.StringFlag{
			Name:  "fifo",
			Usage: "dump data to file or `<fifo>`",
		},

		// { "debug", no_argument, NULL, EXTCAP_OPT_DEBUG}, \
		// { "debug-file", required_argument, NULL, EXTCAP_OPT_DEBUG_FILE}
	}

	if extapp.GetAllConfigOptions != nil {
		opts := extapp.GetAllConfigOptions()
		for _, opt := range opts {
			switch opt.(type) {
			case *ConfigStringOpt:
				app.Flags = append(app.Flags, &cli.StringFlag{
					Name:     opt.call(),
					Usage:    opt.display(),
					Required: opt.isRequired(),
					Value:    opt.(*ConfigStringOpt).defaultValue,
				})
			case *ConfigBoolOpt:
				app.Flags = append(app.Flags, &cli.BoolFlag{
					Name:     opt.call(),
					Usage:    opt.display(),
					Required: opt.isRequired(),
					Value:    opt.(*ConfigBoolOpt).defaultValue,
				})
			case *ConfigIntegerOpt:
				app.Flags = append(app.Flags, &cli.IntFlag{
					Name:     opt.call(),
					Usage:    opt.display(),
					Required: opt.isRequired(),
					Value:    opt.(*ConfigIntegerOpt).defaultValue,
				})
				// case *SelectorConfig:
			default:
				errStr := fmt.Sprintf("Unknown config option type: %T", opt)
				panic(errStr)
			}
		}
	}

	app.Action = extapp.mainAction

	if err := app.Run(arguments); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(-1)
	}
}

func (extapp App) mainAction(ctx *cli.Context) error {

	// Print all interfaces
	if showIface := ctx.IsSet("extcap-interfaces"); showIface {
		ifaces, err := extapp.GetInterfaces()
		if err != nil {
			return err
		}

		fmt.Println(extapp.Version)
		for i := range ifaces {
			fmt.Println(ifaces[i])
		}

		return nil
	}

	// Print DLTs for given interface
	if ctx.IsSet("extcap-dlts") {
		if !ctx.IsSet("extcap-interface") {
			return ErrNoInterfaceSpecified
		}

		iface := ctx.String("extcap-interface")
		dlt, err := extapp.GetDLT(iface)
		if err != nil {
			return err
		}

		fmt.Println(dlt)
		return nil
	}

	// Print config options for given interface
	if ctx.IsSet("extcap-config") {
		// Return immediately in the case if confg options are not supported
		if extapp.GetConfigOptions == nil {
			return nil
		}

		if !ctx.IsSet("extcap-interface") {
			return ErrNoInterfaceSpecified
		}

		iface := ctx.String("extcap-interface")
		opts, err := extapp.GetConfigOptions(iface)
		if err != nil {
			return err
		}

		for i := range opts {
			opts[i].setNumber(i)
			fmt.Println(opts[i])
		}

		return nil
	}

	// Print config options for given interface
	if ctx.IsSet("capture") {
		if !ctx.IsSet("extcap-interface") {
			return ErrNoInterfaceSpecified
		}
		if !ctx.IsSet("fifo") {
			return ErrNoPipeProvided
		}

		iface := ctx.String("extcap-interface")
		fifo := ctx.String("fifo")
		filter := ctx.String("extcap-capture-filter")

		opts := make(map[string]interface{})
		for _, name := range ctx.FlagNames() {
			if name == "extcap-interface" || name == "fifo" || name == "extcap-capture-filter" {
				continue
			}
			opts[name] = ctx.Value(name)
		}

		openPipeFunc := extapp.OpenPipe
		if openPipeFunc == nil {
			openPipeFunc = openPipe
		}

		pipe, err := openPipeFunc(fifo)
		if err != nil {
			return err
		}

		if err = extapp.StartCapture(iface, pipe, filter, opts); err != nil {
			return err
		}

		return nil
	}

	// Validate capture filter
	if ctx.IsSet("extcap-capture-filter") {
		os.Exit(0)
	}

	return cli.ShowAppHelp(ctx)
}

func openPipe(name string) (io.WriteCloser, error) {
	pipe, err := os.OpenFile(name, os.O_WRONLY, os.ModeNamedPipe)
	if err != nil {
		return nil, fmt.Errorf("unable to open pipe: %w", err)
	}

	return pipe, nil
}

const helpTemplate = `NAME:
   {{.Name}}{{if .Usage}} - {{.Usage}}{{end}}

USAGE:
   {{if .UsageText}}{{.UsageText}}{{else}}{{.HelpName}} {{if .VisibleFlags}}[global options]{{end}}{{if .Commands}} command [command options]{{end}} {{if .ArgsUsage}}{{.ArgsUsage}}{{else}}[arguments...]{{end}}{{end}}{{if .Version}}{{if not .HideVersion}}

VERSION:
   {{.Version}}{{end}}{{end}}{{if .Description}}

DESCRIPTION:
   {{.Description}}{{end}}{{if len .Authors}}

AUTHOR{{with $length := len .Authors}}{{if ne 1 $length}}S{{end}}{{end}}:
   {{range $index, $author := .Authors}}{{if $index}}
   {{end}}{{$author}}{{end}}{{end}}{{if .VisibleCommands}}

OPTIONS:
   {{range $index, $option := .VisibleFlags}}{{if $index}}
   {{end}}{{$option}}{{end}}{{end}}{{if .Copyright}}

COPYRIGHT:
   {{.Copyright}}{{end}}
`
