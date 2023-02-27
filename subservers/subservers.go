package subservers

// SubServerName is a name used to identify a particular Lit sub-server.
type SubServerName string

const (
	LND     SubServerName = "lnd"
	LIT     SubServerName = "lit"
	LOOP    SubServerName = "loop"
	POOL    SubServerName = "pool"
	FARADAY SubServerName = "faraday"
)

// String returns the string representation of the sub-server name.
func (s SubServerName) String() string {
	return string(s)
}
