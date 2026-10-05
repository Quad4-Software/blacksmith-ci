package update

type VersionData struct {
	Latest string `json:"latest"`
	Next   string `json:"next"`
	RC     string `json:"rc"`
}

type NewVersion struct {
	Version  string
	AssetURL string
}

type VersionEntry struct {
	Version string   `json:"version"`
	Title   string   `json:"title"`
	Aliases []string `json:"aliases"`
}

const (
	crowVersionURL = "https://docs.crowci.com/versions.json"
	BinaryURL      = "https://codefloe.com/crowci/crow/releases/download/v%s/crow-cli_%s_%s.tar.gz"
)
