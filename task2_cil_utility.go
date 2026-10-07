package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

const baseURL = "https://homeworksite.site"

type Movie struct {
	ID       int    `json:"id"`
	Title    string `json:"title"`
	Year     int    `json:"year"`
	Director string `json:"director"`
}

type Result struct {
	ID    int
	Movie *Movie
	Err   error
}

type Config struct {
	From    int
	To      int
	Workers int
	Timeout time.Duration
}

func main() {
	os.Exit(run())
}

func run() int {
	cfg, err := parseFlags()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	client := &http.Client{}

	results := make(chan Result)

	jobs := make(chan int)

	var wg sync.WaitGroup
	wg.Add(cfg.Workers)

	for i := 0; i < cfg.Workers; i++ {
		go worker(ctx, &wg, client, cfg.Timeout, jobs, results)
	}

	
	go func() {
		defer close(jobs)

		for id := cfg.From; id <= cfg.To; id++ {
			select {
			case jobs <- id:
			case <-ctx.Done():
				return
			}
		}
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	cancelled := false

	for result := range results {
		if result.Err != nil {
			fmt.Fprintf(
				os.Stderr,
				"movie %d: %v\n",
				result.ID,
				result.Err,
			)
			continue
		}

		fmt.Printf(
			"%d — %s — %d — %s\n",
			result.Movie.ID,
			result.Movie.Title,
			result.Movie.Year,
			result.Movie.Director,
		)
	}

	if ctx.Err() != nil {
		cancelled = true
	}

	if cancelled {
		fmt.Fprintln(os.Stderr, "interrupted")
		return 130
	}

	return 0
}

func parseFlags() (Config, error) {
	flagSet := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	var (
		from    int
		to      int
		workers int
		timeout time.Duration
	)

	flagSet.IntVar(&from, "from", 0, "ID первого фильма")
	flagSet.IntVar(&to, "to", 0, "ID последнего фильма")
	flagSet.IntVar(&workers, "workers", 10, "количество workers")
	flagSet.DurationVar(&timeout, "timeout", 5*time.Second, "таймаут одного HTTP-запроса")

	if err := flagSet.Parse(os.Args[1:]); err != nil {
		return Config{}, fmt.Errorf("invalid flags: %w", err)
	}

	if !flagWasProvided(flagSet, "from") {
		return Config{}, errors.New("обязательный флаг --from не указан")
	}

	if !flagWasProvided(flagSet, "to") {
		return Config{}, errors.New("обязательный флаг --to не указан")
	}

	if from > to {
		return Config{}, fmt.Errorf(
			"--from должен быть меньше или равен --to: from=%d, to=%d",
			from,
			to,
		)
	}

	if workers <= 0 {
		return Config{}, fmt.Errorf(
			"--workers должен быть больше 0: %d",
			workers,
		)
	}

	if timeout <= 0 {
		return Config{}, fmt.Errorf(
			"--timeout должен быть больше 0: %s",
			timeout,
		)
	}

	if flagSet.NArg() != 0 {
		return Config{}, fmt.Errorf(
			"неизвестные аргументы: %v",
			flagSet.Args(),
		)
	}

	return Config{
		From:    from,
		To:      to,
		Workers: workers,
		Timeout: timeout,
	}, nil
}

func flagWasProvided(fs *flag.FlagSet, name string) bool {
	found := false

	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})

	return found
}

func worker(
	ctx context.Context,
	wg *sync.WaitGroup,
	client *http.Client,
	timeout time.Duration,
	jobs <-chan int,
	results chan<- Result,
) {
	defer wg.Done()

	for {
		select {
		case <-ctx.Done():
			return

		case id, ok := <-jobs:
			if !ok {
				return
			}

			movie, err := fetchMovie(ctx, client, timeout, id)

			select {
			case results <- Result{
				ID:    id,
				Movie: movie,
				Err:   err,
			}:
			case <-ctx.Done():
				return
			}
		}
	}
}

func fetchMovie(
	parent context.Context,
	client *http.Client,
	timeout time.Duration,
	id int,
) (*Movie, error) {
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	url := fmt.Sprintf("%s/%d/info.0.json", baseURL, id)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, fmt.Errorf("timeout after %s", timeout)
		}

		if errors.Is(ctx.Err(), context.Canceled) {
			return nil, context.Canceled
		}

		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var movie Movie

	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&movie); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	return &movie, nil
}