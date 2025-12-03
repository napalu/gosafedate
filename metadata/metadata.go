package metadata

type Metadata struct {
	Version     string `json:"version"`
	Checksum    string `json:"sha256"`
	Signature   string `json:"signature"`
	DownloadURL string `json:"downloadUrl"`
}
