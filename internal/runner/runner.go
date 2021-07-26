package runner

import (
	"bufio"
	"os"
	"path"

	"github.com/pkg/errors"
	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/notify/pkg/providers"
	"github.com/projectdiscovery/notify/pkg/types"
	"gopkg.in/yaml.v2"
)

// Runner contains the internal logic of the program
type Runner struct {
	options   *types.Options
	providers *providers.Client
}

// NewRunner instance
func NewRunner(options *types.Options) (*Runner, error) {

	var providerOptions providers.Options

	if options.ProviderConfig == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		options.ProviderConfig = path.Join(home, types.DefaultProviderConfigLocation)
		gologger.Print().Msgf("Using default provider config: %s\n", options.ProviderConfig)
	}

	file, err := os.Open(options.ProviderConfig)
	if err != nil {
		return nil, errors.Wrap(err, "could not open provider config file")
	}

	if parseErr := yaml.NewDecoder(file).Decode(&providerOptions); parseErr != nil {
		file.Close()
		return nil, errors.Wrap(parseErr, "could not parse provider config file")
	}

	file.Close()

	prClient, err := providers.New(&providerOptions, options.Providers, options.Profiles)
	if err != nil {
		return nil, err
	}

	return &Runner{options: options, providers: prClient}, nil
}

// Run polling and notification
func (r *Runner) Run() error {

	// If stdin/file input is present pass everything to webhooks and exit
	if hasStdin() || r.options.Data != "" {
		var inFile *os.File
		var err error

		switch {
		case hasStdin():
			inFile = os.Stdin

		case r.options.Data != "":
			inFile, err = os.Open(r.options.Data)
			if err != nil {
				gologger.Fatal().Msgf("%s\n", err)
			}
		}

		if r.options.Bulk {
			fi, err := inFile.Stat()
			if err != nil {
				gologger.Fatal().Msgf("%s\n", err)
			}

			msgB := make([]byte, fi.Size())

			n, err := inFile.Read(msgB)
			if err != nil || n == 0 {
				gologger.Fatal().Msgf("%s\n", err)
			}

			// char limit to search for a split
			searchLimit := 250
			if r.options.CharLimit < searchLimit {
				searchLimit = r.options.CharLimit
			}

			items := SplitText(string(msgB), r.options.CharLimit, searchLimit)

			for _, v := range items {
				if err := r.sendMessage(v); err != nil {
					gologger.Fatal().Msgf("%s\n", err)
				}
			}

			os.Exit(0)
		}

		br := bufio.NewScanner(inFile)
		for br.Scan() {
			msg := br.Text()
			//nolint:errcheck
			r.sendMessage(msg)

		}
		os.Exit(0)
	}
	return errors.New("notify works with stdin or file using -data flag")
}

func (r *Runner) sendMessage(msg string) error {

	if len(msg) > 0 {
		gologger.Print().Msgf(msg)
		err := r.providers.Send(msg)
		if err != nil {
			return err
		}
	}
	return nil
}

// Close the runner instance
func (r *Runner) Close() {}
