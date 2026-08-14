package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/sirrobot01/snagarr/internal/client"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func newLoginCommand() *cobra.Command {
	var options clientOptions
	var username string
	var tokenStdin bool
	var passwordStdin bool

	cmd := &cobra.Command{
		Use:   "login [server]",
		Short: "Connect this CLI to a Snagarr server",
		Long:  "Sign in with your Snagarr username and password, or save an existing capture token.\nThe session is stored in a user-only configuration file.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			hasTokenFlag := strings.TrimSpace(options.token) != ""
			if tokenStdin && hasTokenFlag {
				return fmt.Errorf("--token and --token-stdin cannot be used together")
			}
			if (tokenStdin || hasTokenFlag) && (passwordStdin || username != "") {
				return fmt.Errorf("token and username/password login options cannot be used together")
			}
			server, err := loginServer(options, args)
			if err != nil {
				return err
			}
			base, err := client.New(server, "")
			if err != nil {
				return err
			}

			reader := bufio.NewReader(cmd.InOrStdin())
			token := strings.TrimSpace(options.token)
			var user client.User
			if tokenStdin {
				token, err = readLine(reader)
				if err != nil {
					return fmt.Errorf("read token: %w", err)
				}
				if token == "" {
					return fmt.Errorf("token is required")
				}
			}

			if token != "" {
				authenticated, err := client.New(base.Server(), token)
				if err != nil {
					return err
				}
				user, err = authenticated.Me(cmd.Context())
				if err != nil {
					return fmt.Errorf("verify token: %w", err)
				}
			} else {
				if username == "" {
					fmt.Fprint(cmd.OutOrStdout(), "Username: ")
					username, err = readLine(reader)
					if err != nil {
						return fmt.Errorf("read username: %w", err)
					}
				}
				if username == "" {
					return fmt.Errorf("username is required")
				}
				password, err := readPassword(cmd, reader, passwordStdin)
				if err != nil {
					return err
				}
				session, err := base.Login(cmd.Context(), username, password)
				if err != nil {
					return fmt.Errorf("sign in: %w", err)
				}
				token = session.Token
				user = session.User
			}

			path, err := saveConfig(savedConfig{Server: base.Server(), Token: token})
			if err != nil {
				return err
			}
			p := newPrinter(cmd.OutOrStdout(), options.noColor)
			fmt.Fprintf(cmd.OutOrStdout(), "%s  Logged in as %s\n", p.success("✓"), p.heading("@"+user.Username))
			fmt.Fprintf(cmd.OutOrStdout(), "   %s\n", base.Server())
			fmt.Fprintf(cmd.OutOrStdout(), "   %s\n", p.muted("Saved to "+path))
			return nil
		},
	}
	cmd.Flags().StringVarP(&username, "username", "u", "", "username (prompted when omitted)")
	cmd.Flags().StringVar(&options.token, "token", "", "existing bearer token")
	cmd.Flags().BoolVar(&tokenStdin, "token-stdin", false, "read an existing bearer token from stdin")
	cmd.Flags().BoolVar(&passwordStdin, "password-stdin", false, "read the password from stdin")
	cmd.Flags().BoolVar(&options.noColor, "no-color", false, "disable terminal colors")
	cmd.Flags().StringVar(&options.server, "server", "", "Snagarr server URL")
	return cmd
}

func newLogoutCommand() *cobra.Command {
	var noColor bool
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Remove the saved CLI session",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			config, path, err := loadConfig()
			if err != nil {
				return err
			}
			if config.Token == "" {
				fmt.Fprintln(cmd.OutOrStdout(), "No saved session.")
				return nil
			}
			config.Token = ""
			if _, err := saveConfig(config); err != nil {
				return err
			}
			p := newPrinter(cmd.OutOrStdout(), noColor)
			fmt.Fprintf(cmd.OutOrStdout(), "%s  Logged out\n", p.success("✓"))
			fmt.Fprintln(cmd.OutOrStdout(), p.muted("   Removed the token from "+path))
			return nil
		},
	}
	cmd.Flags().BoolVar(&noColor, "no-color", false, "disable terminal colors")
	return cmd
}

func newSnagCommand() *cobra.Command {
	var options clientOptions
	var note string
	cmd := &cobra.Command{
		Use:     "snag <title, text, or URL>",
		Aliases: []string{"capture", "add"},
		Short:   "Capture something to watch",
		Example: "  snagarr snag \"Sinners\"\n  snagarr snag https://www.imdb.com/title/tt31193180/\n  pbpaste | snagarr snag -",
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query, err := captureQuery(cmd.InOrStdin(), args)
			if err != nil {
				return err
			}
			api, _, err := apiClient(options)
			if err != nil {
				return err
			}
			item, err := api.Capture(cmd.Context(), client.CaptureInput{
				Query: query, Source: "cli", Note: strings.TrimSpace(note),
			})
			if err != nil {
				return friendlyClientError(err, api.Server())
			}
			if options.json {
				return writeJSON(cmd.OutOrStdout(), item)
			}
			newPrinter(cmd.OutOrStdout(), options.noColor).captured(item)
			return nil
		},
	}
	addClientFlags(cmd, &options)
	cmd.Flags().StringVarP(&note, "note", "n", "", "note to keep with the item")
	return cmd
}

func newListCommand() *cobra.Command {
	var options clientOptions
	var status string
	var archived bool
	var limit int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Show the shared list",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !validStatus(status) {
				return fmt.Errorf("unknown status %q; use new, monitored, requested, available, watched, or needs_review", status)
			}
			if limit < 1 || limit > 200 {
				return fmt.Errorf("--limit must be between 1 and 200")
			}
			api, _, err := apiClient(options)
			if err != nil {
				return err
			}
			items, err := api.Items(cmd.Context(), status, archived, limit)
			if err != nil {
				return friendlyClientError(err, api.Server())
			}
			if options.json {
				return writeJSON(cmd.OutOrStdout(), items)
			}
			newPrinter(cmd.OutOrStdout(), options.noColor).list(items, outputWidth(cmd.OutOrStdout()))
			return nil
		},
	}
	addClientFlags(cmd, &options)
	cmd.Flags().StringVarP(&status, "status", "s", "", "filter by exact item status")
	cmd.Flags().BoolVarP(&archived, "archived", "a", false, "show archived items")
	cmd.Flags().IntVarP(&limit, "limit", "l", 50, "maximum number of items")
	return cmd
}

func newStatusCommand() *cobra.Command {
	var options clientOptions
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Check the server and household state",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			api, _, err := apiClient(options)
			if err != nil {
				return err
			}
			status, err := api.Status(cmd.Context())
			if err != nil {
				return friendlyClientError(err, api.Server())
			}
			if options.json {
				return writeJSON(cmd.OutOrStdout(), status)
			}
			newPrinter(cmd.OutOrStdout(), options.noColor).statusView(api.Server(), status)
			return nil
		},
	}
	addClientFlags(cmd, &options)
	return cmd
}

func addClientFlags(cmd *cobra.Command, options *clientOptions) {
	cmd.Flags().StringVar(&options.server, "server", "", "Snagarr server URL (or SNAGARR_URL)")
	cmd.Flags().StringVar(&options.token, "token", "", "bearer token (or SNAGARR_TOKEN)")
	cmd.Flags().BoolVar(&options.json, "json", false, "print JSON for scripts")
	cmd.Flags().BoolVar(&options.noColor, "no-color", false, "disable terminal colors")
}

func apiClient(options clientOptions) (*client.Client, string, error) {
	config, path, err := resolveConfig(options)
	if err != nil {
		return nil, path, err
	}
	api, err := client.New(config.Server, config.Token)
	if err != nil {
		return nil, path, err
	}
	return api, path, nil
}

func loginServer(options clientOptions, args []string) (string, error) {
	if len(args) == 1 {
		return args[0], nil
	}
	config, _, err := resolveConfig(options)
	if err != nil {
		return "", err
	}
	return config.Server, nil
}

func readPassword(cmd *cobra.Command, reader *bufio.Reader, fromStdin bool) (string, error) {
	if fromStdin {
		password, err := readLine(reader)
		if err != nil {
			return "", fmt.Errorf("read password: %w", err)
		}
		if password == "" {
			return "", fmt.Errorf("password is required")
		}
		return password, nil
	}
	input, ok := cmd.InOrStdin().(*os.File)
	if !ok || !term.IsTerminal(int(input.Fd())) {
		return "", fmt.Errorf("password input is not a terminal; use --password-stdin")
	}
	fmt.Fprint(cmd.OutOrStdout(), "Password: ")
	password, err := term.ReadPassword(int(input.Fd()))
	fmt.Fprintln(cmd.OutOrStdout())
	if err != nil {
		return "", fmt.Errorf("read password: %w", err)
	}
	if len(password) == 0 {
		return "", fmt.Errorf("password is required")
	}
	return string(password), nil
}

func readLine(reader *bufio.Reader) (string, error) {
	value, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimSpace(value), nil
}

func captureQuery(input io.Reader, args []string) (string, error) {
	query := strings.TrimSpace(strings.Join(args, " "))
	if len(args) == 1 && args[0] == "-" {
		data, err := io.ReadAll(io.LimitReader(input, 1<<20))
		if err != nil {
			return "", fmt.Errorf("read capture from stdin: %w", err)
		}
		query = strings.TrimSpace(string(data))
	}
	if query == "" {
		return "", fmt.Errorf("capture text cannot be empty")
	}
	return query, nil
}

func friendlyClientError(err error, server string) error {
	var apiErr *client.APIError
	if errors.As(err, &apiErr) && apiErr.Status == 401 {
		return fmt.Errorf("authentication failed; run `snagarr login %s`", server)
	}
	return err
}

func validStatus(status string) bool {
	switch status {
	case "", "new", "monitored", "requested", "available", "watched", "needs_review":
		return true
	default:
		return false
	}
}
