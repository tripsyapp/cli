package cli

var (
	Version = "dev"
	Commit  = ""
	Date    = ""
)

func versionString() string {
	value := "tripsy version " + Version
	if Commit != "" {
		value += " (" + Commit + ")"
	}
	if Date != "" {
		value += " " + Date
	}
	return value
}
