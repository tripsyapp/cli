package output

import (
	"encoding/json"
	"fmt"
	"io"
)

type Breadcrumb struct {
	Action string `json:"action"`
	Cmd    string `json:"cmd"`
}

type Result struct {
	Data        any
	Summary     string
	Breadcrumbs []Breadcrumb
	Human       string
}

type Envelope struct {
	OK          bool         `json:"ok"`
	Data        any          `json:"data,omitempty"`
	Summary     string       `json:"summary,omitempty"`
	Breadcrumbs []Breadcrumb `json:"breadcrumbs,omitempty"`
}

type ErrorEnvelope struct {
	OK    bool   `json:"ok"`
	Error string `json:"error"`
}

type Options struct {
	JSON       bool
	Quiet      bool
	IsTerminal bool
}

func Render(w io.Writer, opts Options, result Result) error {
	if opts.Quiet {
		return writeJSON(w, result.Data)
	}
	if opts.JSON || !opts.IsTerminal {
		return writeJSON(w, Envelope{
			OK:          true,
			Data:        result.Data,
			Summary:     result.Summary,
			Breadcrumbs: result.Breadcrumbs,
		})
	}

	if result.Human != "" {
		_, err := fmt.Fprint(w, result.Human)
		if err != nil {
			return err
		}
		if result.Human[len(result.Human)-1] != '\n' {
			_, err = fmt.Fprintln(w)
		}
		return err
	}
	return writeJSON(w, result.Data)
}

func RenderError(w io.Writer, opts Options, message string) error {
	if opts.JSON || !opts.IsTerminal {
		return writeJSON(w, ErrorEnvelope{OK: false, Error: message})
	}
	_, err := fmt.Fprintf(w, "Error: %s\n", message)
	return err
}

func Pretty(data any) string {
	encoded, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Sprint(data)
	}
	return string(encoded)
}

func writeJSON(w io.Writer, data any) error {
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}
