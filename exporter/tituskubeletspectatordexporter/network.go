package tituskubeletspectatordexporter

import (
	"path"
)

// e.g. /var/lib/titus-inits/6ea18999-e06b-44ce-b9a5-f4ff55883680/ns/net
const titusEnvironmentsPath = "/var/lib/titus-inits"
const networkNamespaceFileName = "ns/net"

func GetNetworkNamespacePath(podName string) (string, error) {
	return path.Join(titusEnvironmentsPath, podName, networkNamespaceFileName), nil
}
