package tituskubeletspectatordexporter

import (
	"io/ioutil"
	"path"
)

// e.g. /var/lib/titus-environments/fe9298a0-1a0b-4cae-b292-5e5982e251db/netns
const titusEnvironmentsPath = "/var/lib/titus-environments"
const networkNamespaceFileName = "netns"

func GetNetworkNamespacePath(podName string) (string, error) {
	netNsPath := path.Join(titusEnvironmentsPath, podName, networkNamespaceFileName)
	if content, err := ioutil.ReadFile(netNsPath); err != nil {
		return "", err
	} else {
		return string(content), nil
	}
}
