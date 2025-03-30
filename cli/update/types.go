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

const (
	crowVersionURL = "https://crowci.dev/version.json"
	BinaryURL      = "https://codeberg.org/crowci/crow/releases/download/v%s/crow-cli_%s_%s.tar.gz"
)
