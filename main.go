package main

import (
	"fmt"
	_ "github.com/pupilcp/go-rabbitmq-consume/boot"
	"github.com/pupilcp/go-rabbitmq-consume/service"
	"github.com/urfave/cli/v2"
	"os"
)

func main() {
	app := &cli.App{
		Commands: []*cli.Command{
			{
				Name:    "start",
				Aliases: []string{"s"},
				Usage:   "start server",
				Action: func(c *cli.Context) error {
					service.AppService.Run("start")
					return nil
				},
			},
			{
				Name:    "stop",
				Aliases: []string{},
				Usage:   "stop server smoothly",
				Action: func(c *cli.Context) error {
					service.AppService.Run("stop")
					return nil
				},
			},
			{
				Name:    "status",
				Aliases: []string{},
				Usage:   "view server status",
				Action: func(c *cli.Context) error {
					service.AppService.Run("status")
					return nil
				},
			},
		},
	}
	err := app.Run(os.Args)
	if err != nil {
		fmt.Println(err)
	}
}
