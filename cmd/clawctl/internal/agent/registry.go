package agent

import "fmt"

// ClawSpec describes a Claw type (picoclaw, openclaw, zero, etc.).
type ClawSpec struct {
	// Repo is the GitHub repository for release downloads (e.g. "sipeed/picoclaw").
	Repo string
	// DefaultPort is the gateway default listen port.
	DefaultPort int
	// BinaryNames lists all binaries shipped in a release for this Claw type.
	BinaryNames []string
	// GatewayBinary is the binary used to start the gateway process.
	GatewayBinary string
	// GatewayArgs are the arguments passed to the gateway binary.
	// e.g. ["gateway", "-E"] for picoclaw, ["gateway", "start"] for zero.
	GatewayArgs []string
}

// registry holds all known Claw types.
var registry = map[string]ClawSpec{
	"picoclaw": {
		Repo:           "sipeed/picoclaw",
		DefaultPort:    18790,
		BinaryNames:    []string{"picoclaw", "picoclaw-launcher", "picoclaw-launcher-tui"},
		GatewayBinary:  "picoclaw",
		GatewayArgs:    []string{"gateway", "-E"},
	},
	"openclaw": {
		Repo:           "sipeed/openclaw",
		DefaultPort:    18791,
		BinaryNames:    []string{"openclaw", "openclaw-launcher"},
		GatewayBinary:  "openclaw-launcher",
		GatewayArgs:    []string{"gateway"},
	},
	"zero": {
		Repo:           "zeroclaw-labs/zeroclaw",
		DefaultPort:    18792,
		BinaryNames:    []string{"zeroclaw"},
		GatewayBinary:  "zeroclaw",
		GatewayArgs:    []string{"daemon"},
	},
}

// KnownTypes returns the list of all registered Claw type names.
func KnownTypes() []string {
	types := make([]string, 0, len(registry))
	for t := range registry {
		types = append(types, t)
	}
	return types
}

// Get returns the ClawSpec for a given type, or an error if unknown.
func Get(t string) (ClawSpec, error) {
	spec, ok := registry[t]
	if !ok {
		return ClawSpec{}, fmt.Errorf("unknown claw type: %q (known: %v)", t, KnownTypes())
	}
	return spec, nil
}

// MustGet returns the ClawSpec for a given type, panics if unknown.
func MustGet(t string) ClawSpec {
	spec, err := Get(t)
	if err != nil {
		panic(err)
	}
	return spec
}
