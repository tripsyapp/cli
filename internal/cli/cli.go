package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tripsyapp/tripsy-cli/internal/api"
	"github.com/tripsyapp/tripsy-cli/internal/config"
	"github.com/tripsyapp/tripsy-cli/internal/output"
	"github.com/tripsyapp/tripsy-cli/internal/terminal"
)

type app struct {
	args   []string
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer

	root   rootOptions
	store  *config.Store
	client *api.Client
	out    output.Options
}

type rootOptions struct {
	JSON      bool
	Quiet     bool
	APIBase   string
	Token     string
	ConfigDir string
	Help      bool
	Agent     bool
}

func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	root, remaining, err := parseRootFlags(args)
	if err != nil {
		_ = output.RenderError(stderr, output.Options{JSON: root.JSON, Quiet: root.Quiet, IsTerminal: isTerminal(stderr)}, err.Error())
		return 2
	}

	store := config.NewStore(root.ConfigDir)
	credentials, err := store.LoadCredentials()
	if err != nil {
		_ = output.RenderError(stderr, output.Options{JSON: root.JSON, Quiet: root.Quiet, IsTerminal: isTerminal(stderr)}, err.Error())
		return 1
	}

	baseURL := firstNonEmpty(root.APIBase, os.Getenv("TRIPSY_API_BASE"), credentials.BaseURL, api.DefaultBaseURL)
	token := firstNonEmpty(root.Token, os.Getenv("TRIPSY_TOKEN"), credentials.Token)

	a := &app{
		args:   remaining,
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
		root:   root,
		store:  store,
		client: api.NewClient(baseURL, token),
		out: output.Options{
			JSON:       root.JSON,
			Quiet:      root.Quiet,
			IsTerminal: isTerminal(stdout),
		},
	}

	if err := a.execute(context.Background()); err != nil {
		return a.fail(err)
	}
	return 0
}

func (a *app) execute(ctx context.Context) error {
	if len(a.args) == 0 {
		return a.render(output.Result{Data: commandCatalog(), Summary: "Tripsy CLI commands", Human: rootHelp()})
	}

	if a.root.Help {
		return a.handleHelp()
	}

	switch a.args[0] {
	case "help":
		a.args = append(a.args[:0], a.args[1:]...)
		return a.handleHelp()
	case "auth":
		return a.auth(ctx, a.args[1:])
	case "me":
		return a.me(ctx, a.args[1:])
	case "trips", "trip":
		return a.trips(ctx, a.args[1:])
	case "hostings", "hosting":
		return a.resource(ctx, hostingResource, a.args[1:])
	case "activities", "activity":
		return a.resource(ctx, activityResource, a.args[1:])
	case "transportations", "transportation":
		return a.resource(ctx, transportationResource, a.args[1:])
	case "expenses", "expense":
		return a.resource(ctx, expenseResource, a.args[1:])
	case "collaborators":
		return a.collaborators(ctx, a.args[1:])
	case "emails", "email":
		return a.emails(ctx, a.args[1:])
	case "inbox":
		return a.inbox(ctx, a.args[1:])
	case "documents", "document", "docs":
		return a.documents(ctx, a.args[1:])
	case "uploads", "upload":
		return a.uploads(ctx, a.args[1:])
	case "request":
		return a.rawRequest(ctx, a.args[1:])
	case "commands":
		return a.render(output.Result{Data: commandCatalog(), Summary: "Tripsy CLI commands", Human: commandsHuman()})
	case "doctor":
		return a.doctor(ctx, a.args[1:])
	default:
		return usageError("unknown command %q", a.args[0])
	}
}

func (a *app) handleHelp() error {
	if a.root.Agent {
		if len(a.args) == 0 {
			return a.render(output.Result{Data: commandCatalog(), Summary: "Agent command catalog"})
		}
		spec, ok := findCommand(a.args[0])
		if !ok {
			return usageError("unknown command %q", a.args[0])
		}
		return a.render(output.Result{Data: spec, Summary: spec.Summary})
	}

	if len(a.args) == 0 {
		return a.render(output.Result{Data: commandCatalog(), Summary: "Tripsy CLI commands", Human: rootHelp()})
	}
	spec, ok := findCommand(a.args[0])
	if !ok {
		return usageError("unknown command %q", a.args[0])
	}
	return a.render(output.Result{Data: spec, Summary: spec.Summary, Human: commandHelp(spec)})
}

func (a *app) render(result output.Result) error {
	return output.Render(a.stdout, a.out, result)
}

func (a *app) fail(err error) int {
	message := userFacingError(err)
	_ = output.RenderError(a.stderr, output.Options{JSON: a.root.JSON, Quiet: a.root.Quiet, IsTerminal: isTerminal(a.stderr)}, message)
	if errors.As(err, &usageErr{}) {
		return 2
	}
	return 1
}

func parseRootFlags(args []string) (rootOptions, []string, error) {
	var opts rootOptions
	remaining := make([]string, 0, len(args))

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--":
			remaining = append(remaining, args[i+1:]...)
			return opts, remaining, nil
		case arg == "--json":
			opts.JSON = true
		case arg == "--quiet":
			opts.Quiet = true
		case arg == "--help" || arg == "-h":
			opts.Help = true
		case arg == "--agent":
			opts.Agent = true
		case arg == "--api-base":
			value, next, err := requireFlagValue(args, i, arg)
			if err != nil {
				return opts, nil, err
			}
			opts.APIBase = value
			i = next
		case strings.HasPrefix(arg, "--api-base="):
			opts.APIBase = strings.TrimPrefix(arg, "--api-base=")
		case arg == "--token":
			value, next, err := requireFlagValue(args, i, arg)
			if err != nil {
				return opts, nil, err
			}
			opts.Token = value
			i = next
		case strings.HasPrefix(arg, "--token="):
			opts.Token = strings.TrimPrefix(arg, "--token=")
		case arg == "--config-dir":
			value, next, err := requireFlagValue(args, i, arg)
			if err != nil {
				return opts, nil, err
			}
			opts.ConfigDir = value
			i = next
		case strings.HasPrefix(arg, "--config-dir="):
			opts.ConfigDir = strings.TrimPrefix(arg, "--config-dir=")
		default:
			remaining = append(remaining, arg)
		}
	}
	return opts, remaining, nil
}

func requireFlagValue(args []string, index int, flag string) (string, int, error) {
	next := index + 1
	if next >= len(args) {
		return "", index, usageError("%s requires a value", flag)
	}
	return args[next], next, nil
}

func isTerminal(w io.Writer) bool {
	file, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	return err == nil && (info.Mode()&os.ModeCharDevice) != 0
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

type usageErr struct {
	message string
}

func (e usageErr) Error() string {
	return e.message
}

func usageError(format string, args ...any) error {
	return usageErr{message: fmt.Sprintf(format, args...)}
}

func userFacingError(err error) string {
	var apiErr *api.Error
	if errors.As(err, &apiErr) {
		if message := extractAPIMessage(apiErr.Data); message != "" {
			return fmt.Sprintf("HTTP %d: %s", apiErr.StatusCode, message)
		}
	}
	return err.Error()
}

func extractAPIMessage(data any) string {
	switch value := data.(type) {
	case map[string]any:
		for _, key := range []string{"detail", "error"} {
			if item, ok := value[key]; ok {
				return fmt.Sprint(item)
			}
		}
		if item, ok := value["non_field_errors"]; ok {
			return formatValue(item)
		}
		return formatValue(value)
	case string:
		return strings.TrimSpace(value)
	default:
		return ""
	}
}

type flagSet struct {
	values      map[string][]string
	positionals []string
}

func parseFlags(args []string, boolFlags ...string) (*flagSet, error) {
	bools := map[string]bool{}
	for _, name := range boolFlags {
		bools[name] = true
	}
	fs := &flagSet{values: map[string][]string{}}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			fs.positionals = append(fs.positionals, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "--") || arg == "-" {
			fs.positionals = append(fs.positionals, arg)
			continue
		}

		nameValue := strings.TrimPrefix(arg, "--")
		name, value, hasValue := strings.Cut(nameValue, "=")
		if name == "" {
			return nil, usageError("invalid empty flag")
		}
		if !hasValue {
			if bools[name] {
				value = "true"
				hasValue = true
			} else {
				if i+1 >= len(args) {
					return nil, usageError("--%s requires a value", name)
				}
				i++
				value = args[i]
			}
		}
		fs.values[name] = append(fs.values[name], value)
	}

	return fs, nil
}

func (fs *flagSet) Has(name string) bool {
	_, ok := fs.values[name]
	return ok
}

func (fs *flagSet) String(name string) string {
	values := fs.values[name]
	if len(values) == 0 {
		return ""
	}
	return values[len(values)-1]
}

func (fs *flagSet) All(name string) []string {
	return append([]string(nil), fs.values[name]...)
}

func (fs *flagSet) Bool(name string) bool {
	value := fs.String(name)
	return value == "true" || value == "1" || strings.EqualFold(value, "yes")
}

func (fs *flagSet) TripID() string {
	return firstNonEmpty(fs.String("trip"), fs.String("in"))
}

func buildPayload(fs *flagSet, allowed []string) (map[string]any, error) {
	payload := map[string]any{}

	if raw := fs.String("data"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &payload); err != nil {
			return nil, fmt.Errorf("--data must be a JSON object: %w", err)
		}
	}

	for _, field := range allowed {
		flagName := strings.ReplaceAll(field, "_", "-")
		if fs.Has(flagName) {
			payload[field] = parseValue(fs.String(flagName))
		}
	}

	for _, pair := range fs.All("set") {
		key, value, ok := strings.Cut(pair, "=")
		if !ok || strings.TrimSpace(key) == "" {
			return nil, usageError("--set must use key=value")
		}
		payload[key] = parseValue(value)
	}

	return payload, nil
}

func parseValue(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	var parsed any
	decoder := json.NewDecoder(strings.NewReader(value))
	decoder.UseNumber()
	if err := decoder.Decode(&parsed); err == nil {
		return parsed
	}
	return value
}

func commonListQuery(fs *flagSet) url.Values {
	query := url.Values{}
	if fs.Bool("deleted") {
		query.Set("deleted", "true")
	}
	if value := fs.String("updated-since"); value != "" {
		query.Set("updatedSince", value)
	}
	if value := fs.String("fields"); value != "" {
		query.Set("fields", value)
	}
	if value := firstNonEmpty(fs.String("fields-exclude"), fs.String("without-fields")); value != "" {
		query.Set("fields!", value)
	}
	return query
}

func requireToken(client *api.Client) error {
	if strings.TrimSpace(client.Token) == "" {
		return errors.New("not authenticated; run `tripsy auth login` or set TRIPSY_TOKEN")
	}
	return nil
}

func requestData(resp *api.Response) any {
	if resp == nil {
		return nil
	}
	return resp.Data
}

func formatValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			parts = append(parts, fmt.Sprint(item))
		}
		return strings.Join(parts, ", ")
	default:
		encoded, err := json.Marshal(typed)
		if err != nil {
			return fmt.Sprint(typed)
		}
		return string(encoded)
	}
}

func objectMap(data any) map[string]any {
	item, _ := data.(map[string]any)
	return item
}

func results(data any) []any {
	if root, ok := data.(map[string]any); ok {
		if items, ok := root["results"].([]any); ok {
			return items
		}
	}
	if items, ok := data.([]any); ok {
		return items
	}
	return nil
}

func valueString(item any, key string) string {
	object, ok := item.(map[string]any)
	if !ok {
		return ""
	}
	value, ok := object[key]
	if !ok || value == nil {
		return ""
	}
	return fmt.Sprint(value)
}

func formatObjects(title string, data any, columns ...string) string {
	items := results(data)
	if len(items) == 0 {
		return "No " + strings.ToLower(title) + ".\n"
	}

	var b strings.Builder
	b.WriteString(title)
	b.WriteString("\n")
	for _, item := range items {
		parts := make([]string, 0, len(columns))
		for _, column := range columns {
			if value := valueString(item, column); value != "" {
				parts = append(parts, value)
			}
		}
		if len(parts) == 0 {
			parts = append(parts, formatValue(item))
		}
		b.WriteString("  ")
		b.WriteString(strings.Join(parts, "  "))
		b.WriteString("\n")
	}
	return b.String()
}

func formatObject(title string, data any, fields ...string) string {
	object, ok := data.(map[string]any)
	if !ok {
		return output.Pretty(data) + "\n"
	}

	var b strings.Builder
	if title != "" {
		b.WriteString(title)
		b.WriteString("\n")
	}
	for _, field := range fields {
		value, ok := object[field]
		if !ok || value == nil {
			continue
		}
		b.WriteString("  ")
		b.WriteString(field)
		b.WriteString(": ")
		b.WriteString(fmt.Sprint(value))
		b.WriteString("\n")
	}
	return b.String()
}

func formatFullObject(title string, data any, preferredFields ...string) string {
	object, ok := data.(map[string]any)
	if !ok {
		return output.Pretty(data) + "\n"
	}

	var b strings.Builder
	if title != "" {
		b.WriteString(title)
		b.WriteString("\n")
	}
	for _, key := range orderedKeys(object, preferredFields) {
		b.WriteString("  ")
		b.WriteString(key)
		b.WriteString(":")
		value := object[key]
		if isDetailScalar(value) {
			b.WriteString(" ")
			b.WriteString(formatDetailScalar(value))
			b.WriteString("\n")
			continue
		}
		b.WriteString("\n")
		b.WriteString(indent(output.Pretty(value), "    "))
		b.WriteString("\n")
	}
	return b.String()
}

func orderedKeys(object map[string]any, preferredFields []string) []string {
	seen := map[string]bool{}
	keys := make([]string, 0, len(object))
	for _, key := range preferredFields {
		if _, ok := object[key]; ok {
			keys = append(keys, key)
			seen[key] = true
		}
	}

	remaining := make([]string, 0, len(object)-len(keys))
	for key := range object {
		if !seen[key] {
			remaining = append(remaining, key)
		}
	}
	sort.Strings(remaining)
	return append(keys, remaining...)
}

func isDetailScalar(value any) bool {
	switch value.(type) {
	case nil, string, bool, int, int64, float64, json.Number:
		return true
	default:
		return false
	}
}

func formatDetailScalar(value any) string {
	if value == nil {
		return "null"
	}
	return fmt.Sprint(value)
}

func indent(value, prefix string) string {
	lines := strings.Split(strings.TrimRight(value, "\n"), "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}

func (a *app) auth(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return usageError("auth requires a subcommand: login, logout, status, token, reset-password, change-password")
	}

	switch args[0] {
	case "login":
		fs, err := parseFlags(args[1:])
		if err != nil {
			return err
		}
		username := firstNonEmpty(fs.String("username"), fs.String("email"))
		if username == "" && len(fs.positionals) > 0 {
			username = fs.positionals[0]
		}
		if username == "" {
			return usageError("auth login requires --username")
		}

		password := firstNonEmpty(fs.String("password"), os.Getenv("TRIPSY_PASSWORD"))
		if password == "" {
			fmt.Fprint(a.stderr, "Password: ")
			value, hidden, err := terminal.ReadPassword(a.stdin)
			if hidden {
				fmt.Fprintln(a.stderr)
			}
			if err != nil {
				return err
			}
			password = value
		}
		if password == "" {
			return usageError("auth login requires --password or TRIPSY_PASSWORD")
		}

		publicClient := api.NewClient(a.client.BaseURL, "")
		resp, err := publicClient.Request(ctx, "POST", "/auth", nil, map[string]any{
			"username": username,
			"password": password,
		})
		if err != nil {
			return err
		}
		token := firstNonEmpty(valueString(resp.Data, "token"), valueString(resp.Data, "key"))
		if token == "" {
			return errors.New("login response did not include a token")
		}
		if err := a.store.SaveCredentials(config.Credentials{Token: token, BaseURL: savedBaseURL(a.root.APIBase)}); err != nil {
			return err
		}
		a.client.Token = token
		return a.render(output.Result{
			Data:    map[string]any{"authenticated": true},
			Summary: "Logged in",
			Human:   "Logged in to Tripsy.\n",
		})

	case "logout":
		fs, err := parseFlags(args[1:], "local")
		if err != nil {
			return err
		}
		if !fs.Bool("local") && strings.TrimSpace(a.client.Token) != "" {
			_, _ = a.client.Request(ctx, "POST", "/auth/logout/", nil, nil)
		}
		if err := a.store.ClearCredentials(); err != nil {
			return err
		}
		a.client.Token = ""
		return a.render(output.Result{
			Data:    map[string]any{"authenticated": false},
			Summary: "Logged out",
			Human:   "Logged out of Tripsy.\n",
		})

	case "status":
		if strings.TrimSpace(a.client.Token) == "" {
			return a.render(output.Result{
				Data:    map[string]any{"authenticated": false, "api_base": a.client.BaseURL},
				Summary: "Not logged in",
				Human:   "Not logged in.\n",
			})
		}
		resp, err := a.client.Request(ctx, "GET", "/v1/me", nil, nil)
		if err != nil {
			return err
		}
		return a.render(output.Result{
			Data:    resp.Data,
			Summary: "Authenticated",
			Human:   formatObject("Authenticated as", resp.Data, "id", "name", "email", "username", "timezone"),
		})

	case "token":
		return a.authToken(args[1:])

	case "reset-password":
		fs, err := parseFlags(args[1:])
		if err != nil {
			return err
		}
		email := fs.String("email")
		if email == "" && len(fs.positionals) > 0 {
			email = fs.positionals[0]
		}
		if email == "" {
			return usageError("auth reset-password requires --email")
		}
		publicClient := api.NewClient(a.client.BaseURL, "")
		resp, err := publicClient.Request(ctx, "POST", "/auth/password/reset/", nil, map[string]any{"email": email})
		if err != nil {
			return err
		}
		return a.render(output.Result{Data: resp.Data, Summary: "Password reset requested", Human: "Password reset email requested.\n"})

	case "change-password":
		if err := requireToken(a.client); err != nil {
			return err
		}
		fs, err := parseFlags(args[1:])
		if err != nil {
			return err
		}
		newPassword := firstNonEmpty(fs.String("new-password"), fs.String("password"))
		confirm := firstNonEmpty(fs.String("confirm-password"), fs.String("confirm"), newPassword)
		if newPassword == "" {
			return usageError("auth change-password requires --new-password")
		}
		resp, err := a.client.Request(ctx, "POST", "/auth/password/change/", nil, map[string]any{
			"new_password1": newPassword,
			"new_password2": confirm,
		})
		if err != nil {
			return err
		}
		return a.render(output.Result{Data: resp.Data, Summary: "Password changed", Human: "Password changed.\n"})
	default:
		return usageError("unknown auth subcommand %q", args[0])
	}
}

func (a *app) authToken(args []string) error {
	if len(args) > 0 && args[0] == "set" {
		fs, err := parseFlags(args[1:])
		if err != nil {
			return err
		}
		token := fs.String("token")
		if token == "" && len(fs.positionals) > 0 {
			token = fs.positionals[0]
		}
		if token == "" {
			return usageError("auth token set requires a token")
		}
		if err := a.store.SaveCredentials(config.Credentials{Token: token, BaseURL: savedBaseURL(a.root.APIBase)}); err != nil {
			return err
		}
		a.client.Token = token
		return a.render(output.Result{Data: map[string]any{"configured": true}, Summary: "Token saved", Human: "Token saved.\n"})
	}

	if strings.TrimSpace(a.client.Token) == "" {
		return errors.New("no Tripsy token configured")
	}
	return a.render(output.Result{
		Data:    map[string]any{"token": a.client.Token},
		Summary: "Token",
		Human:   a.client.Token + "\n",
	})
}

func savedBaseURL(value string) string {
	if strings.TrimSpace(value) == "" || strings.TrimRight(value, "/") == api.DefaultBaseURL {
		return ""
	}
	return strings.TrimRight(value, "/")
}

func (a *app) me(ctx context.Context, args []string) error {
	if err := requireToken(a.client); err != nil {
		return err
	}
	if len(args) == 0 {
		args = []string{"show"}
	}

	switch args[0] {
	case "show", "get":
		resp, err := a.client.Request(ctx, "GET", "/v1/me", nil, nil)
		if err != nil {
			return err
		}
		return a.render(output.Result{
			Data:    resp.Data,
			Summary: "Current user",
			Human:   formatObject("Current user", resp.Data, "id", "name", "email", "username", "timezone", "default_currency"),
		})
	case "update", "patch":
		fs, err := parseFlags(args[1:])
		if err != nil {
			return err
		}
		payload, err := buildPayload(fs, meFields)
		if err != nil {
			return err
		}
		if len(payload) == 0 {
			return usageError("me update requires --set key=value or a supported field flag")
		}
		resp, err := a.client.Request(ctx, "PATCH", "/v1/me", nil, payload)
		if err != nil {
			return err
		}
		return a.render(output.Result{
			Data:    resp.Data,
			Summary: "Profile updated",
			Human:   formatObject("Updated profile", resp.Data, "id", "name", "email", "username", "timezone", "default_currency"),
		})
	default:
		return usageError("unknown me subcommand %q", args[0])
	}
}

func (a *app) trips(ctx context.Context, args []string) error {
	if err := requireToken(a.client); err != nil {
		return err
	}
	if len(args) == 0 {
		args = []string{"list"}
	}

	switch args[0] {
	case "list", "ls":
		fs, err := parseFlags(args[1:], "deleted")
		if err != nil {
			return err
		}
		query := commonListQuery(fs)
		resp, err := a.client.Request(ctx, "GET", "/v1/trips", query, nil)
		if err != nil {
			return err
		}
		return a.render(output.Result{
			Data:    resp.Data,
			Summary: fmt.Sprintf("%d trips", len(results(resp.Data))),
			Breadcrumbs: []output.Breadcrumb{
				{Action: "show", Cmd: "tripsy trips show <id>"},
				{Action: "activities", Cmd: "tripsy activities list --trip <id>"},
				{Action: "hostings", Cmd: "tripsy hostings list --trip <id>"},
				{Action: "transportations", Cmd: "tripsy transportations list --trip <id>"},
			},
			Human: formatObjects("Trips", resp.Data, "id", "name", "starts_at", "ends_at", "timezone"),
		})
	case "show", "get":
		fs, err := parseFlags(args[1:])
		if err != nil {
			return err
		}
		id, err := positionalID(fs, "trips show requires a trip id")
		if err != nil {
			return err
		}
		resp, err := a.client.Request(ctx, "GET", "/v1/trips/"+id, nil, nil)
		if err != nil {
			return err
		}
		return a.render(output.Result{
			Data:    resp.Data,
			Summary: "Trip " + id,
			Breadcrumbs: []output.Breadcrumb{
				{Action: "activities", Cmd: "tripsy activities list --trip " + id},
				{Action: "documents", Cmd: "tripsy documents attach --trip " + id + " --url <url> --title <title>"},
			},
			Human: formatFullObject("Trip", resp.Data, tripDetailFields...),
		})
	case "create", "new":
		fs, err := parseFlags(args[1:])
		if err != nil {
			return err
		}
		payload, err := buildPayload(fs, tripFields)
		if err != nil {
			return err
		}
		if len(payload) == 0 {
			return usageError("trips create requires --name or --data")
		}
		resp, err := a.client.Request(ctx, "POST", "/v1/trips", nil, payload)
		if err != nil {
			return err
		}
		id := valueString(resp.Data, "id")
		return a.render(output.Result{
			Data:    resp.Data,
			Summary: "Trip created",
			Breadcrumbs: []output.Breadcrumb{
				{Action: "show", Cmd: "tripsy trips show " + firstNonEmpty(id, "<id>")},
			},
			Human: formatObject("Created trip", resp.Data, "id", "name", "starts_at", "ends_at", "timezone"),
		})
	case "update", "patch":
		fs, err := parseFlags(args[1:])
		if err != nil {
			return err
		}
		id, err := positionalID(fs, "trips update requires a trip id")
		if err != nil {
			return err
		}
		payload, err := buildPayload(fs, tripFields)
		if err != nil {
			return err
		}
		if len(payload) == 0 {
			return usageError("trips update requires --set key=value or a supported field flag")
		}
		resp, err := a.client.Request(ctx, "PATCH", "/v1/trips/"+id, nil, payload)
		if err != nil {
			return err
		}
		return a.render(output.Result{Data: resp.Data, Summary: "Trip updated", Human: formatObject("Updated trip", resp.Data, "id", "name", "starts_at", "ends_at", "timezone")})
	case "delete", "rm":
		fs, err := parseFlags(args[1:])
		if err != nil {
			return err
		}
		id, err := positionalID(fs, "trips delete requires a trip id")
		if err != nil {
			return err
		}
		resp, err := a.client.Request(ctx, "DELETE", "/v1/trips/"+id, nil, nil)
		if err != nil {
			return err
		}
		return a.render(output.Result{Data: requestData(resp), Summary: "Trip deleted", Human: "Trip deleted.\n"})
	default:
		return usageError("unknown trips subcommand %q", args[0])
	}
}

type resourceSpec struct {
	Name         string
	Plural       string
	Singular     string
	ListPath     string
	DetailPath   string
	FilterFlag   string
	FilterParam  string
	Fields       []string
	DetailFields []string
	Columns      []string
}

var hostingResource = resourceSpec{
	Name:         "hosting",
	Plural:       "hostings",
	Singular:     "hosting",
	ListPath:     "/v1/trip/%s/hostings",
	DetailPath:   "/v1/trip/%s/hosting/%s",
	Fields:       hostingFields,
	DetailFields: hostingDetailFields,
	Columns:      []string{"id", "name", "starts_at", "ends_at", "address"},
}

var activityResource = resourceSpec{
	Name:         "activity",
	Plural:       "activities",
	Singular:     "activity",
	ListPath:     "/v1/trip/%s/activities",
	DetailPath:   "/v1/trip/%s/activity/%s",
	FilterFlag:   "activity-type",
	FilterParam:  "activityType",
	Fields:       activityFields,
	DetailFields: activityDetailFields,
	Columns:      []string{"id", "name", "activity_type", "starts_at", "ends_at"},
}

var transportationResource = resourceSpec{
	Name:         "transportation",
	Plural:       "transportations",
	Singular:     "transportation",
	ListPath:     "/v1/trip/%s/transportations",
	DetailPath:   "/v1/trip/%s/transportation/%s",
	FilterFlag:   "transportation-type",
	FilterParam:  "transportationType",
	Fields:       transportationFields,
	DetailFields: transportationDetailFields,
	Columns:      []string{"id", "name", "transportation_type", "departure_at", "arrival_at"},
}

var expenseResource = resourceSpec{
	Name:         "expense",
	Plural:       "expenses",
	Singular:     "expense",
	ListPath:     "/v1/trip/%s/expenses",
	DetailPath:   "/v1/trip/%s/expense/%s",
	Fields:       expenseFields,
	DetailFields: expenseDetailFields,
	Columns:      []string{"id", "title", "date", "price", "currency"},
}

func (a *app) resource(ctx context.Context, spec resourceSpec, args []string) error {
	if err := requireToken(a.client); err != nil {
		return err
	}
	if len(args) == 0 {
		args = []string{"list"}
	}

	switch args[0] {
	case "list", "ls":
		fs, err := parseFlags(args[1:], "deleted")
		if err != nil {
			return err
		}
		tripID := fs.TripID()
		if tripID == "" {
			return usageError("%s list requires --trip", spec.Plural)
		}
		query := commonListQuery(fs)
		if spec.FilterFlag != "" && fs.String(spec.FilterFlag) != "" {
			query.Set(spec.FilterParam, fs.String(spec.FilterFlag))
		}
		resp, err := a.client.Request(ctx, "GET", fmt.Sprintf(spec.ListPath, tripID), query, nil)
		if err != nil {
			return err
		}
		return a.render(output.Result{
			Data:    resp.Data,
			Summary: fmt.Sprintf("%d %s", len(results(resp.Data)), spec.Plural),
			Breadcrumbs: []output.Breadcrumb{
				{Action: "show", Cmd: fmt.Sprintf("tripsy %s show --trip %s <id>", spec.Plural, tripID)},
				{Action: "create", Cmd: fmt.Sprintf("tripsy %s create --trip %s --name <name>", spec.Plural, tripID)},
			},
			Human: formatObjects(title(spec.Plural), resp.Data, spec.Columns...),
		})
	case "show", "get":
		fs, err := parseFlags(args[1:])
		if err != nil {
			return err
		}
		tripID := fs.TripID()
		if tripID == "" {
			return usageError("%s show requires --trip", spec.Plural)
		}
		id, err := positionalID(fs, fmt.Sprintf("%s show requires an id", spec.Plural))
		if err != nil {
			return err
		}
		resp, err := a.client.Request(ctx, "GET", fmt.Sprintf(spec.DetailPath, tripID, id), nil, nil)
		if err != nil {
			return err
		}
		return a.render(output.Result{
			Data:    resp.Data,
			Summary: spec.Singular + " " + id,
			Breadcrumbs: []output.Breadcrumb{
				{Action: "attach-document", Cmd: fmt.Sprintf("tripsy documents attach --trip %s --parent %s:%s --url <url> --title <title>", tripID, spec.Singular, id)},
			},
			Human: formatFullObject(title(spec.Singular), resp.Data, spec.DetailFields...),
		})
	case "create", "new":
		fs, err := parseFlags(args[1:])
		if err != nil {
			return err
		}
		tripID := fs.TripID()
		if tripID == "" {
			return usageError("%s create requires --trip", spec.Plural)
		}
		payload, err := buildPayload(fs, spec.Fields)
		if err != nil {
			return err
		}
		if len(payload) == 0 {
			return usageError("%s create requires --data or field flags", spec.Plural)
		}
		resp, err := a.client.Request(ctx, "POST", fmt.Sprintf(spec.ListPath, tripID), nil, payload)
		if err != nil {
			return err
		}
		id := valueString(resp.Data, "id")
		return a.render(output.Result{
			Data:    resp.Data,
			Summary: title(spec.Singular) + " created",
			Breadcrumbs: []output.Breadcrumb{
				{Action: "show", Cmd: fmt.Sprintf("tripsy %s show --trip %s %s", spec.Plural, tripID, firstNonEmpty(id, "<id>"))},
			},
			Human: formatObject("Created "+spec.Singular, resp.Data, "id", "name", "title", "starts_at", "ends_at", "departure_at", "arrival_at", "price", "currency"),
		})
	case "update", "patch":
		fs, err := parseFlags(args[1:])
		if err != nil {
			return err
		}
		tripID := fs.TripID()
		if tripID == "" {
			return usageError("%s update requires --trip", spec.Plural)
		}
		id, err := positionalID(fs, fmt.Sprintf("%s update requires an id", spec.Plural))
		if err != nil {
			return err
		}
		payload, err := buildPayload(fs, append(spec.Fields, "update_trip"))
		if err != nil {
			return err
		}
		if len(payload) == 0 {
			return usageError("%s update requires --set key=value or field flags", spec.Plural)
		}
		resp, err := a.client.Request(ctx, "PATCH", fmt.Sprintf(spec.DetailPath, tripID, id), nil, payload)
		if err != nil {
			return err
		}
		return a.render(output.Result{Data: resp.Data, Summary: title(spec.Singular) + " updated", Human: formatObject("Updated "+spec.Singular, resp.Data, "id", "name", "title", "starts_at", "ends_at", "departure_at", "arrival_at", "price", "currency")})
	case "delete", "rm":
		fs, err := parseFlags(args[1:])
		if err != nil {
			return err
		}
		tripID := fs.TripID()
		if tripID == "" {
			return usageError("%s delete requires --trip", spec.Plural)
		}
		id, err := positionalID(fs, fmt.Sprintf("%s delete requires an id", spec.Plural))
		if err != nil {
			return err
		}
		resp, err := a.client.Request(ctx, "DELETE", fmt.Sprintf(spec.DetailPath, tripID, id), nil, nil)
		if err != nil {
			return err
		}
		return a.render(output.Result{Data: requestData(resp), Summary: title(spec.Singular) + " deleted", Human: title(spec.Singular) + " deleted.\n"})
	default:
		return usageError("unknown %s subcommand %q", spec.Plural, args[0])
	}
}

func positionalID(fs *flagSet, message string) (string, error) {
	if len(fs.positionals) == 0 || strings.TrimSpace(fs.positionals[0]) == "" {
		return "", usageError("%s", message)
	}
	return fs.positionals[0], nil
}

func (a *app) collaborators(ctx context.Context, args []string) error {
	if err := requireToken(a.client); err != nil {
		return err
	}
	fs, err := parseFlags(args)
	if err != nil {
		return err
	}
	tripID := fs.TripID()
	if tripID == "" && len(fs.positionals) > 0 {
		tripID = fs.positionals[0]
	}
	if tripID == "" {
		return usageError("collaborators requires --trip")
	}
	resp, err := a.client.Request(ctx, "GET", "/v1/trip/"+tripID+"/collaborators", nil, nil)
	if err != nil {
		return err
	}
	return a.render(output.Result{
		Data:    resp.Data,
		Summary: fmt.Sprintf("%d collaborators", len(results(resp.Data))),
		Human:   formatObjects("Collaborators", resp.Data, "id", "name", "email", "joined"),
	})
}

func (a *app) emails(ctx context.Context, args []string) error {
	if err := requireToken(a.client); err != nil {
		return err
	}
	if len(args) == 0 {
		args = []string{"list"}
	}

	switch args[0] {
	case "list", "ls":
		resp, err := a.client.Request(ctx, "GET", "/v1/emails", nil, nil)
		if err != nil {
			return err
		}
		return a.render(output.Result{Data: resp.Data, Summary: fmt.Sprintf("%d emails", len(results(resp.Data))), Human: formatObjects("Emails", resp.Data, "id", "email", "verified")})
	case "add":
		fs, err := parseFlags(args[1:])
		if err != nil {
			return err
		}
		email := fs.String("email")
		if email == "" && len(fs.positionals) > 0 {
			email = fs.positionals[0]
		}
		if email == "" {
			return usageError("emails add requires --email")
		}
		resp, err := a.client.Request(ctx, "POST", "/v1/emails/add", nil, map[string]any{"email": email})
		if err != nil {
			return err
		}
		return a.render(output.Result{Data: resp.Data, Summary: "Email added", Human: "Email added; check your inbox for verification.\n"})
	case "delete", "rm":
		fs, err := parseFlags(args[1:])
		if err != nil {
			return err
		}
		id, err := positionalID(fs, "emails delete requires an email id")
		if err != nil {
			return err
		}
		resp, err := a.client.Request(ctx, "DELETE", "/v1/emails/"+id, nil, nil)
		if err != nil {
			return err
		}
		return a.render(output.Result{Data: resp.Data, Summary: "Email deleted", Human: "Email deleted.\n"})
	default:
		return usageError("unknown emails subcommand %q", args[0])
	}
}

func (a *app) inbox(ctx context.Context, args []string) error {
	if err := requireToken(a.client); err != nil {
		return err
	}
	if len(args) == 0 {
		args = []string{"list"}
	}

	switch args[0] {
	case "list", "ls":
		resp, err := a.client.Request(ctx, "GET", "/v1/automation/emails", nil, nil)
		if err != nil {
			return err
		}
		return a.render(output.Result{
			Data:    resp.Data,
			Summary: fmt.Sprintf("%d inbox emails", len(results(resp.Data))),
			Breadcrumbs: []output.Breadcrumb{
				{Action: "show", Cmd: "tripsy inbox show <id>"},
				{Action: "move", Cmd: "tripsy inbox update <id> --trip-id <trip-id>"},
			},
			Human: formatObjects("Inbox", resp.Data, "id", "date", "subject", "attachments_count"),
		})
	case "show", "get":
		fs, err := parseFlags(args[1:])
		if err != nil {
			return err
		}
		id, err := positionalID(fs, "inbox show requires an email id")
		if err != nil {
			return err
		}
		resp, err := a.client.Request(ctx, "GET", "/v1/automation/emails/"+id, nil, nil)
		if err != nil {
			return err
		}
		return a.render(output.Result{Data: resp.Data, Summary: "Inbox email " + id, Human: formatObject("Inbox email", resp.Data, "id", "date", "subject", "body_preview", "attachments_count")})
	case "update", "move", "patch":
		fs, err := parseFlags(args[1:])
		if err != nil {
			return err
		}
		id, err := positionalID(fs, "inbox update requires an email id")
		if err != nil {
			return err
		}
		payload, err := buildPayload(fs, []string{"subject", "trip_id", "activity_id", "hosting_id", "transportation_id"})
		if err != nil {
			return err
		}
		if len(payload) == 0 {
			return usageError("inbox update requires --subject or a move target flag")
		}
		resp, err := a.client.Request(ctx, "PATCH", "/v1/automation/emails/"+id, nil, payload)
		if err != nil {
			return err
		}
		return a.render(output.Result{Data: requestData(resp), Summary: "Inbox email updated", Human: "Inbox email updated.\n"})
	case "delete", "rm":
		fs, err := parseFlags(args[1:])
		if err != nil {
			return err
		}
		id, err := positionalID(fs, "inbox delete requires an email id")
		if err != nil {
			return err
		}
		resp, err := a.client.Request(ctx, "DELETE", "/v1/automation/emails/"+id, nil, nil)
		if err != nil {
			return err
		}
		return a.render(output.Result{Data: resp.Data, Summary: "Inbox email deleted", Human: "Inbox email deleted.\n"})
	default:
		return usageError("unknown inbox subcommand %q", args[0])
	}
}

func (a *app) documents(ctx context.Context, args []string) error {
	if err := requireToken(a.client); err != nil {
		return err
	}
	if len(args) == 0 {
		return usageError("documents requires a subcommand: get, update, attach, upload, delete")
	}

	switch args[0] {
	case "get", "download-url":
		fs, err := parseFlags(args[1:])
		if err != nil {
			return err
		}
		id, err := positionalID(fs, "documents get requires a document id")
		if err != nil {
			return err
		}
		resp, err := a.client.Request(ctx, "GET", "/v1/documents/"+id+"/get", nil, nil)
		if err != nil {
			return err
		}
		return a.render(output.Result{Data: resp.Data, Summary: "Document URL", Human: formatObject("Document URL", resp.Data, "title", "file_type", "download_url", "expires_at")})
	case "update", "move", "patch":
		fs, err := parseFlags(args[1:])
		if err != nil {
			return err
		}
		id, err := positionalID(fs, "documents update requires a document id")
		if err != nil {
			return err
		}
		payload, err := buildPayload(fs, []string{"title", "trip_id", "activity_id", "hosting_id", "transportation_id"})
		if err != nil {
			return err
		}
		if len(payload) == 0 {
			return usageError("documents update requires --title or a move target flag")
		}
		resp, err := a.client.Request(ctx, "PATCH", "/v1/documents/"+id, nil, payload)
		if err != nil {
			return err
		}
		return a.render(output.Result{Data: requestData(resp), Summary: "Document updated", Human: "Document updated.\n"})
	case "attach", "add":
		fs, err := parseFlags(args[1:])
		if err != nil {
			return err
		}
		tripID := fs.TripID()
		if tripID == "" {
			return usageError("documents attach requires --trip")
		}
		parentType, parentID, err := parentFromFlags(fs, tripID)
		if err != nil {
			return err
		}
		payload, err := buildPayload(fs, documentFields)
		if err != nil {
			return err
		}
		if payload["url"] == nil {
			return usageError("documents attach requires --url")
		}
		if payload["file_type"] == nil {
			payload["file_type"] = "url"
		}
		resp, err := a.client.Request(ctx, "POST", documentAttachPath(tripID, parentType, parentID), nil, payload)
		if err != nil {
			return err
		}
		return a.render(output.Result{Data: resp.Data, Summary: "Document attached", Human: formatObject("Attached document", resp.Data, "id", "title", "file_type", "url")})
	case "upload":
		return a.uploadDocument(ctx, args[1:])
	case "delete", "rm":
		fs, err := parseFlags(args[1:])
		if err != nil {
			return err
		}
		tripID := fs.TripID()
		if tripID == "" {
			return usageError("documents delete requires --trip")
		}
		parentType, parentID, err := parentFromFlags(fs, tripID)
		if err != nil {
			return err
		}
		id, err := positionalID(fs, "documents delete requires a document id")
		if err != nil {
			return err
		}
		path := documentAttachPath(tripID, parentType, parentID) + "/" + id
		resp, err := a.client.Request(ctx, "DELETE", path, nil, nil)
		if err != nil {
			return err
		}
		return a.render(output.Result{Data: requestData(resp), Summary: "Document deleted", Human: "Document deleted.\n"})
	default:
		return usageError("unknown documents subcommand %q", args[0])
	}
}

func (a *app) uploadDocument(ctx context.Context, args []string) error {
	fs, err := parseFlags(args)
	if err != nil {
		return err
	}
	if len(fs.positionals) == 0 {
		return usageError("documents upload requires a file path")
	}
	filePath := fs.positionals[0]
	tripID := fs.TripID()
	if tripID == "" {
		return usageError("documents upload requires --trip")
	}
	parentType, parentID, err := parentFromFlags(fs, tripID)
	if err != nil {
		return err
	}

	info, err := os.Stat(filePath)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return usageError("documents upload requires a file, not a directory")
	}

	contentType := firstNonEmpty(fs.String("file-type"), fs.String("content-type"), mime.TypeByExtension(filepath.Ext(filePath)), "application/octet-stream")
	filename := firstNonEmpty(fs.String("filename"), filepath.Base(filePath))
	uploadPayload := map[string]any{
		"purpose":        "document",
		"parent_type":    parentType,
		"parent_id":      parseValue(parentID),
		"filename":       filename,
		"content_type":   contentType,
		"content_length": info.Size(),
	}
	uploadResp, err := a.client.Request(ctx, "POST", "/v1/storage/uploads", nil, uploadPayload)
	if err != nil {
		return err
	}
	uploadInfo := objectMap(uploadResp.Data)
	uploadURL := fmt.Sprint(uploadInfo["upload_url"])
	if uploadURL == "" || uploadURL == "<nil>" {
		return errors.New("storage upload response did not include upload_url")
	}
	headers := mapString(uploadInfo["headers"])
	if err := a.client.UploadFile(ctx, uploadURL, headers, filePath); err != nil {
		return err
	}

	title := firstNonEmpty(fs.String("title"), filename)
	attachPayload := map[string]any{
		"url":         uploadInfo["object_key"],
		"file_type":   contentType,
		"title":       title,
		"description": fs.String("description"),
	}
	attachResp, err := a.client.Request(ctx, "POST", documentAttachPath(tripID, parentType, parentID), nil, attachPayload)
	if err != nil {
		return err
	}
	return a.render(output.Result{Data: attachResp.Data, Summary: "Document uploaded", Human: formatObject("Uploaded document", attachResp.Data, "id", "title", "file_type", "url")})
}

func (a *app) uploads(ctx context.Context, args []string) error {
	if len(args) == 0 {
		args = []string{"create"}
	}
	if args[0] != "create" {
		return usageError("uploads only supports the create subcommand")
	}
	fs, err := parseFlags(args[1:])
	if err != nil {
		return err
	}
	payload, err := buildPayload(fs, []string{"purpose", "filename", "content_type", "visibility", "content_length", "parent_type", "parent_id"})
	if err != nil {
		return err
	}
	if payload["purpose"] == nil || payload["filename"] == nil || payload["content_type"] == nil {
		return usageError("uploads create requires --purpose, --filename, and --content-type")
	}
	if payload["purpose"] == "document" {
		if err := requireToken(a.client); err != nil {
			return err
		}
	}
	resp, err := a.client.Request(ctx, "POST", "/v1/storage/uploads", nil, payload)
	if err != nil {
		return err
	}
	return a.render(output.Result{Data: resp.Data, Summary: "Upload URL created", Human: formatObject("Upload URL", resp.Data, "upload_url", "method", "object_key", "public_url", "expires_at")})
}

func (a *app) rawRequest(ctx context.Context, args []string) error {
	fs, err := parseFlags(args)
	if err != nil {
		return err
	}
	if len(fs.positionals) < 2 {
		return usageError("request requires METHOD and PATH")
	}
	method := strings.ToUpper(fs.positionals[0])
	path := fs.positionals[1]
	query := url.Values{}
	for _, pair := range fs.All("query") {
		key, value, ok := strings.Cut(pair, "=")
		if !ok {
			return usageError("--query must use key=value")
		}
		query.Add(key, value)
	}
	var body any
	if raw := fs.String("data"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &body); err != nil {
			return fmt.Errorf("--data must be JSON: %w", err)
		}
	}
	if body == nil && len(fs.All("set")) > 0 {
		body, err = buildPayload(fs, nil)
		if err != nil {
			return err
		}
	}
	resp, err := a.client.Request(ctx, method, path, query, body)
	if err != nil {
		return err
	}
	return a.render(output.Result{Data: resp.Data, Summary: fmt.Sprintf("HTTP %d", resp.StatusCode), Human: output.Pretty(resp.Data) + "\n"})
}

func (a *app) doctor(ctx context.Context, args []string) error {
	fs, err := parseFlags(args, "verbose")
	if err != nil {
		return err
	}
	data := map[string]any{
		"api_base":         a.client.BaseURL,
		"config_dir":       a.store.Dir,
		"credentials_path": a.store.CredentialsPath(),
		"has_token":        strings.TrimSpace(a.client.Token) != "",
	}
	if strings.TrimSpace(a.client.Token) != "" {
		resp, err := a.client.Request(ctx, "GET", "/v1/me", nil, nil)
		if err != nil {
			data["api_check"] = userFacingError(err)
		} else {
			data["api_check"] = "ok"
			if fs.Bool("verbose") {
				data["me"] = resp.Data
			}
		}
	}
	return a.render(output.Result{
		Data:    data,
		Summary: "Doctor complete",
		Human:   doctorHuman(data),
	})
}

func parentFromFlags(fs *flagSet, tripID string) (string, string, error) {
	parent := fs.String("parent")
	if parent == "" {
		return "trip", tripID, nil
	}
	parentType, parentID, ok := strings.Cut(parent, ":")
	if !ok || parentType == "" || parentID == "" {
		return "", "", usageError("--parent must use type:id, for example trip:42 or activity:202")
	}
	switch parentType {
	case "trip", "activity", "hosting", "transportation":
		return parentType, parentID, nil
	default:
		return "", "", usageError("unsupported document parent type %q", parentType)
	}
}

func documentAttachPath(tripID, parentType, parentID string) string {
	switch parentType {
	case "trip":
		return "/v1/trip/" + tripID + "/documents"
	case "activity":
		return "/v1/trip/" + tripID + "/activity/" + parentID + "/documents"
	case "hosting":
		return "/v1/trip/" + tripID + "/hosting/" + parentID + "/documents"
	case "transportation":
		return "/v1/trip/" + tripID + "/transportation/" + parentID + "/documents"
	default:
		return "/v1/trip/" + tripID + "/documents"
	}
}

func mapString(value any) map[string]string {
	result := map[string]string{}
	object, ok := value.(map[string]any)
	if !ok {
		return result
	}
	for key, item := range object {
		result[key] = fmt.Sprint(item)
	}
	return result
}

func doctorHuman(data map[string]any) string {
	var b strings.Builder
	b.WriteString("Tripsy CLI doctor\n")
	for _, key := range []string{"api_base", "config_dir", "credentials_path", "has_token", "api_check"} {
		if value, ok := data[key]; ok {
			b.WriteString("  ")
			b.WriteString(key)
			b.WriteString(": ")
			b.WriteString(fmt.Sprint(value))
			b.WriteString("\n")
		}
	}
	return b.String()
}

func title(value string) string {
	if value == "" {
		return ""
	}
	return strings.ToUpper(value[:1]) + value[1:]
}

var meFields = []string{
	"photo_url",
	"username",
	"email",
	"name",
	"language",
	"calendar_emojis",
	"calendar_hidden_categories",
	"calendar_lodgings_all_day",
	"calendar_trips_all_day",
	"is_premium",
	"timezone",
	"default_currency",
	"store_currency",
	"asa_campaign_name",
	"asa_keyword_name",
	"asa_store_country",
	"notifications_flights_general_enabled",
	"notifications_flights_departure_and_arrival",
	"notifications_collaborators_new_activities_enabled",
}

var tripFields = []string{
	"internal_identifier",
	"name",
	"timezone",
	"hidden",
	"description",
	"starts_at",
	"ends_at",
	"cover_gradient",
	"cover_image_url",
	"has_dates",
	"number_of_days",
	"guest_invites",
}

var tripDetailFields = append([]string{"id"}, append(tripFields,
	"collaborators_count",
	"owner",
	"collaborators",
	"documents",
	"guests",
)...)

var hostingFields = []string{
	"internal_identifier",
	"hidden",
	"starts_at",
	"ends_at",
	"timezone",
	"name",
	"description",
	"address",
	"longitude",
	"latitude",
	"phone",
	"apple_maps_id",
	"room_type",
	"room_number",
	"website",
	"notes",
	"provider_location_url",
	"provider_document",
	"provider_reservation_code",
	"provider_reservation_description",
	"google_places_id",
	"price",
	"currency",
	"assigned_users",
}

var hostingDetailFields = append([]string{
	"id",
	"owner",
	"trip",
}, append(hostingFields,
	"emails",
	"documents",
	"created_at",
	"updated_at",
)...)

var activityFields = []string{
	"internal_identifier",
	"hidden",
	"activity_type",
	"period",
	"starts_at",
	"ends_at",
	"all_day",
	"name",
	"description",
	"phone",
	"website",
	"checked",
	"address",
	"longitude",
	"latitude",
	"notes",
	"apple_maps_id",
	"timezone",
	"provider_location_url",
	"provider_document",
	"provider_reservation_code",
	"provider_reservation_description",
	"google_places_id",
	"price",
	"currency",
	"assigned_users",
}

var activityDetailFields = append([]string{
	"id",
	"owner",
	"trip",
}, append(activityFields,
	"documents",
	"emails",
	"created_at",
	"updated_at",
)...)

var transportationFields = []string{
	"internal_identifier",
	"hidden",
	"name",
	"description",
	"notes",
	"transportation_type",
	"phone",
	"website",
	"departure_description",
	"departure_at",
	"departure_timezone",
	"departure_address",
	"departure_longitude",
	"departure_latitude",
	"arrival_description",
	"arrival_at",
	"arrival_timezone",
	"arrival_address",
	"arrival_longitude",
	"arrival_latitude",
	"company",
	"seat_number",
	"seat_class",
	"transport_number",
	"actual_transport_number",
	"coach_number",
	"vehicle_description",
	"departure_terminal",
	"departure_gate",
	"arrival_terminal",
	"arrival_gate",
	"arrival_bags",
	"flight_aware_identifier",
	"automatic_updates",
	"provider_url",
	"provider_document",
	"provider_reservation_code",
	"provider_reservation_description",
	"distance_meters",
	"price",
	"currency",
	"assigned_users",
	"departure_apple_maps_id",
}

var transportationDetailFields = append([]string{
	"id",
	"owner",
	"trip",
}, append(transportationFields,
	"documents",
	"emails",
	"created_at",
	"updated_at",
)...)

var expenseFields = []string{
	"title",
	"date",
	"price",
	"currency",
}

var expenseDetailFields = append([]string{"id"}, append(expenseFields,
	"owner",
	"trip",
)...)

var documentFields = []string{
	"url",
	"thumb_url",
	"favicon_url",
	"file_type",
	"title",
	"description",
}

type commandSpec struct {
	Name        string   `json:"name"`
	Usage       string   `json:"usage"`
	Summary     string   `json:"summary"`
	Subcommands []string `json:"subcommands,omitempty"`
	Examples    []string `json:"examples,omitempty"`
	Gotchas     []string `json:"gotchas,omitempty"`
}

func commandCatalog() []commandSpec {
	return []commandSpec{
		{
			Name:        "auth",
			Usage:       "tripsy auth <login|logout|status|token|reset-password|change-password>",
			Summary:     "Authenticate with Tripsy and manage the stored API token.",
			Subcommands: []string{"login", "logout", "status", "token", "reset-password", "change-password"},
			Examples: []string{
				"tripsy auth login --username you@example.com",
				"tripsy auth token set YOUR_TOKEN",
				"tripsy auth status",
			},
		},
		{
			Name:        "me",
			Usage:       "tripsy me <show|update>",
			Summary:     "Read or update the current Tripsy profile.",
			Subcommands: []string{"show", "update"},
			Examples: []string{
				"tripsy me show",
				"tripsy me update --name 'Updated Name' --timezone America/Sao_Paulo",
			},
		},
		{
			Name:        "trips",
			Usage:       "tripsy trips <list|show|create|update|delete>",
			Summary:     "List, create, inspect, update, and soft-delete trips.",
			Subcommands: []string{"list", "show", "create", "update", "delete"},
			Examples: []string{
				"tripsy trips list",
				"tripsy trips create --name Italy --starts-at 2026-06-01 --ends-at 2026-06-15 --timezone Europe/Rome",
				"tripsy trips update 42 --description 'Summer vacation'",
			},
			Gotchas: []string{"GET /v1/trips returns a custom envelope with results but no count."},
		},
		resourceCommandSpec("hostings", "hosting", "Hotel and lodging plans", "tripsy hostings create --trip 42 --name 'Hotel Eden' --starts-at 2026-06-01T14:00:00Z"),
		resourceCommandSpec("activities", "activity", "Scheduled or unscheduled trip activities", "tripsy activities create --trip 42 --name 'Colosseum Tour' --activity-type sightseeing"),
		resourceCommandSpec("transportations", "transportation", "Flights, trains, cars, and other transport", "tripsy transportations create --trip 42 --name 'Flight to Rome' --transportation-type airplane"),
		resourceCommandSpec("expenses", "expense", "Trip expenses", "tripsy expenses create --trip 42 --title Dinner --price 78.5 --currency EUR"),
		{
			Name:        "collaborators",
			Usage:       "tripsy collaborators --trip <trip-id>",
			Summary:     "List collaborators and pending invitations for a trip.",
			Examples:    []string{"tripsy collaborators --trip 42"},
			Subcommands: []string{"list"},
		},
		{
			Name:        "emails",
			Usage:       "tripsy emails <list|add|delete>",
			Summary:     "Manage alternative email addresses.",
			Subcommands: []string{"list", "add", "delete"},
			Examples:    []string{"tripsy emails add work@example.com"},
		},
		{
			Name:        "inbox",
			Usage:       "tripsy inbox <list|show|update|delete>",
			Summary:     "Review automation emails that still need manual handling.",
			Subcommands: []string{"list", "show", "update", "delete"},
			Examples:    []string{"tripsy inbox update 55 --trip-id 42", "tripsy inbox update 55 --subject 'Renamed itinerary email'"},
		},
		{
			Name:        "documents",
			Usage:       "tripsy documents <get|update|attach|upload|delete>",
			Summary:     "Get download URLs, move documents, attach links, and upload files.",
			Subcommands: []string{"get", "update", "attach", "upload", "delete"},
			Examples: []string{
				"tripsy documents get 280258",
				"tripsy documents attach --trip 42 --url https://tripsy.app --title Tripsy",
				"tripsy documents upload boarding-pass.pdf --trip 42 --parent transportation:303",
			},
			Gotchas: []string{"For uploaded private files, the CLI creates a signed upload, PUTs bytes to S3, then attaches the returned object_key."},
		},
		{
			Name:        "uploads",
			Usage:       "tripsy uploads create --purpose <document|profile_photo|trip_cover> --filename <name> --content-type <mime>",
			Summary:     "Create a raw backend-signed S3 upload URL.",
			Subcommands: []string{"create"},
			Examples:    []string{"tripsy uploads create --purpose trip_cover --filename cover.jpg --content-type image/jpeg"},
		},
		{
			Name:     "request",
			Usage:    "tripsy request METHOD PATH [--query key=value] [--data JSON]",
			Summary:  "Make a raw Tripsy API request for endpoints not yet wrapped by a friendly command.",
			Examples: []string{"tripsy request GET /v1/me", "tripsy request PATCH /v1/me --data '{\"timezone\":\"Europe/Rome\"}'"},
		},
		{
			Name:     "commands",
			Usage:    "tripsy commands [--json]",
			Summary:  "Print the command catalog for humans or agents.",
			Examples: []string{"tripsy commands --json", "tripsy trips --help --agent"},
		},
		{
			Name:     "doctor",
			Usage:    "tripsy doctor [--verbose]",
			Summary:  "Check config, token presence, and authenticated API access.",
			Examples: []string{"tripsy doctor", "tripsy doctor --verbose --json"},
		},
	}
}

func resourceCommandSpec(plural, singular, summary, createExample string) commandSpec {
	return commandSpec{
		Name:        plural,
		Usage:       fmt.Sprintf("tripsy %s <list|show|create|update|delete> --trip <trip-id>", plural),
		Summary:     summary + ".",
		Subcommands: []string{"list", "show", "create", "update", "delete"},
		Examples: []string{
			fmt.Sprintf("tripsy %s list --trip 42", plural),
			createExample,
			fmt.Sprintf("tripsy %s update --trip 42 <id> --set notes='Updated notes'", plural),
		},
		Gotchas: []string{fmt.Sprintf("Use --trip for every %s command because the public API scopes %s under a trip.", singular, plural)},
	}
}

func findCommand(name string) (commandSpec, bool) {
	for _, command := range commandCatalog() {
		if command.Name == name {
			return command, true
		}
	}
	return commandSpec{}, false
}

func rootHelp() string {
	return strings.TrimSpace(`Tripsy CLI

Usage:
  tripsy [--json] [--quiet] [--api-base URL] [--token TOKEN] <command>

Commands:
  auth              Authenticate and manage tokens
  me                Show or update your Tripsy profile
  trips             Manage trips
  hostings          Manage lodging plans inside a trip
  activities        Manage trip activities
  transportations   Manage flights, trains, cars, and other transport
  expenses          Manage trip expenses
  collaborators     List trip collaborators
  emails            Manage alternative email addresses
  inbox             Review unprocessed automation emails
  documents         Attach, move, upload, and read documents
  uploads           Create raw signed upload URLs
  request           Make a raw API request
  commands          Print the command catalog
  doctor            Diagnose local CLI state

Output:
  Human output is used in terminals. JSON envelopes are used with --json or when piped.
  Use --quiet for raw JSON data without the envelope.

Configuration:
  Credentials are stored in ~/.config/tripsy-cli/credentials.json.
  TRIPSY_TOKEN and TRIPSY_API_BASE override stored values.
`) + "\n"
}

func commandsHuman() string {
	var b strings.Builder
	b.WriteString("Commands\n")
	for _, command := range commandCatalog() {
		b.WriteString("  ")
		b.WriteString(command.Name)
		b.WriteString(" - ")
		b.WriteString(command.Summary)
		b.WriteString("\n")
	}
	return b.String()
}

func commandHelp(command commandSpec) string {
	var b strings.Builder
	b.WriteString(command.Name)
	b.WriteString("\n\nUsage:\n  ")
	b.WriteString(command.Usage)
	b.WriteString("\n\n")
	b.WriteString(command.Summary)
	b.WriteString("\n")
	if len(command.Subcommands) > 0 {
		b.WriteString("\nSubcommands:\n")
		for _, subcommand := range command.Subcommands {
			b.WriteString("  ")
			b.WriteString(subcommand)
			b.WriteString("\n")
		}
	}
	if len(command.Examples) > 0 {
		b.WriteString("\nExamples:\n")
		for _, example := range command.Examples {
			b.WriteString("  ")
			b.WriteString(example)
			b.WriteString("\n")
		}
	}
	if len(command.Gotchas) > 0 {
		b.WriteString("\nGotchas:\n")
		for _, gotcha := range command.Gotchas {
			b.WriteString("  ")
			b.WriteString(gotcha)
			b.WriteString("\n")
		}
	}
	return b.String()
}
