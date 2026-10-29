package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strings"

	"github.com/pixelwave/grok-manager/internal/app"
)

var Version = "26.10.29.11"

func version() string {
	body, err := os.ReadFile("VERSION")
	if err == nil {
		if value := strings.TrimSpace(string(body)); value != "" {
			return value
		}
	}
	if strings.TrimSpace(Version) != "" {
		return strings.TrimSpace(Version)
	}
	return "0"
}

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "up":
			if err := app.StartService(); err != nil {
				fmt.Fprintf(os.Stderr, "Could not start Grok Manager.\n")
				os.Exit(1)
			}
			fmt.Print(app.StatusText(version()))
			return
		case "down":
			if err := app.StopService(); err != nil {
				fmt.Fprintf(os.Stderr, "Could not stop Grok Manager.\n")
				os.Exit(1)
			}
			fmt.Print(app.StatusText(version()))
			return
		case "status":
			fmt.Print(app.StatusText(version()))
			return
		case "update":
			latest, err := app.ApplyUpdate(version())
			if err != nil && err.Error() == "already up to date" {
				fmt.Printf("Grok Manager %s is up to date.\n", version())
				return
			}
			if err != nil {
				fmt.Fprintf(os.Stderr, "Could not update Grok Manager.\n")
				os.Exit(1)
			}
			fmt.Printf("Updated to %s.\n", latest)
			app.RestartAfterUpdate()
			return
		case "install":
			fs := flag.NewFlagSet("install", flag.ExitOnError)
			port := fs.Int("p", 18090, "port")
			_ = fs.Parse(os.Args[2:])
			if err := app.SavePort(*port); err != nil {
				fmt.Fprintf(os.Stderr, "Could not save the port.\n")
				os.Exit(1)
			}
			if err := os.Setenv("GROK_MANAGER_PORT", fmt.Sprint(*port)); err != nil {
				os.Exit(1)
			}
			if err := app.InstallService(*port); err != nil {
				fmt.Fprintf(os.Stderr, "Could not install the service.\n")
				os.Exit(1)
			}
			fmt.Printf("Grok Manager is listening on port %d\n", *port)
			fmt.Printf("For more info use \"grok-manager status\"\n")
			if runtime.GOOS == "windows" {
				fmt.Printf("Open a new terminal so the grok-manager command is available.\n")
			}
			return
		case "uninstall":
			purge := false
			for _, arg := range os.Args[2:] {
				if arg == "--purge" {
					purge = true
				}
			}
			if err := app.Uninstall(purge); err != nil {
				fmt.Fprintf(os.Stderr, "Could not uninstall Grok Manager.\n")
				os.Exit(1)
			}
			fmt.Println("Grok Manager was removed.")
			return
		}
	}
	app.CleanLeftovers()
	app.HideServerConsole()
	port := app.LoadPort()
	server := app.Server{Version: version(), Port: port}
	stop, err := app.Announce(app.MachineName(), port, version())
	if err == nil {
		defer stop()
	}
	address := fmt.Sprintf(":%d", port)
	fmt.Printf("Grok Manager listening on http://127.0.0.1:%d\n", port)
	if err := http.ListenAndServe(address, server.Handler()); err != nil {
		fmt.Fprintf(os.Stderr, "Could not listen on port %d.\n", port)
		os.Exit(1)
	}
}
