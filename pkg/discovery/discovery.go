package discovery

// Subject is the NATS subject for discovery pings.
const Subject = "signal.discovery.ping"

// ServiceInfo describes a running service instance.
type ServiceInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Endpoint    string `json:"endpoint"`
	Health      string `json:"health"`
	Description string `json:"description"`
}
