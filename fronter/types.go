package fronter

// MultiAccountConfig holds the essential relay parameters,
// decoupled from the full GUI config struct to avoid circular imports.
type MultiAccountConfig struct {
	GoogleIP    string
	FrontDomain string
	AuthKey     string
	ScriptIDs   []string
	VerifySSL   bool
}