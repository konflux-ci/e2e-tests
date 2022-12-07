package status

// Health payload
type Health struct {
	Alive       bool   `json:"alive"`
	Environment string `json:"environment"`
	Revision    string `json:"revision"`
	BuildTime   string `json:"buildTime"`
	StartTime   string `json:"startTime"`
}
