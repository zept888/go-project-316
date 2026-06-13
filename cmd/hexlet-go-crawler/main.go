package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"code/crawler"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:  "hexlet-go-crawler",
		Usage: "analyze a website structure",
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:  "depth",
				Usage: "crawl depth",
				Value: 10,
			},
			&cli.IntFlag{
				Name:  "retries",
				Usage: "number of retries for failed requests",
				Value: 1,
			},
			&cli.DurationFlag{
				Name:  "delay",
				Usage: "delay between requests (example: 200ms, 1s)",
				Value: 0,
			},
			&cli.DurationFlag{
				Name:  "timeout",
				Usage: "per-request timeout",
				Value: 15 * time.Second,
			},
			&cli.Float64Flag{
				Name:  "rps",
				Usage: "limit requests per second (overrides delay)",
				Value: 0,
			},
			&cli.StringFlag{
				Name:  "user-agent",
				Usage: "custom user agent",
			},
			&cli.IntFlag{
				Name:  "workers",
				Usage: "number of concurrent workers",
				Value: 4,
			},
			&cli.BoolFlag{
				Name:  "indent-json",
				Usage: "pretty-print JSON report",
			},
		},
		Action: func(c *cli.Context) error {
			if c.NArg() < 1 {
				return fmt.Errorf("URL is required")
			}

			opts := crawler.Options{
				URL:         c.Args().First(),
				Depth:       c.Int("depth"),
				Retries:     c.Int("retries"),
				Delay:       c.Duration("delay"),
				RPS:         c.Float64("rps"),
				Timeout:     c.Duration("timeout"),
				UserAgent:   c.String("user-agent"),
				Concurrency: c.Int("workers"),
				IndentJSON:  c.Bool("indent-json"),
				HTTPClient:  &http.Client{},
			}

			report, err := crawler.Analyze(context.Background(), opts)
			if err != nil {
				return err
			}

			if len(report) == 0 || report[len(report)-1] != '\n' {
				report = append(report, '\n')
			}
			_, err = os.Stdout.Write(report)
			return err
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
