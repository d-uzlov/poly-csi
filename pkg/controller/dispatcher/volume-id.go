package dispatcher

import (
	"fmt"
	"strings"
)

const volumePrefix = "v"
const snapshotPrefix = "s"

func createVolumeId(serverId string, volumeName string) string {
	if serverId == "" {
		panic("serverId is empty")
	}
	if volumeName == "" {
		panic("volumeName is empty")
	}
	if strings.Contains(serverId, "/") {
		panic("serverId must not contain '/' symbol")
	}
	if strings.Contains(volumeName, "/") {
		panic("volumeName must not contain '/' symbol")
	}
	return volumePrefix + "/" + serverId + "/" + volumeName
}

func parseVolumeId(id string) (serverId string, volumeName string, err error) {
	parts := strings.Split(id, "/")
	if len(parts) != 3 {
		return "", "", fmt.Errorf("expected format 'v/serverId/volumeName' but found %v slash-separated components: %v", len(parts), id)
	}
	if parts[0] != volumePrefix {
		return "", "", fmt.Errorf("expected format 'v/serverId/volumeName' but id starts with different prefix: %v", id)
	}
	serverId = parts[1]
	volumeName = parts[2]
	if serverId == "" {
		return "", "", fmt.Errorf("expected format 'v/serverId/volumeName' but serverId is empty: %v", id)
	}
	if volumeName == "" {
		return "", "", fmt.Errorf("expected format 'v/serverId/volumeName' but volumeName is empty: %v", id)
	}
	return
}

func createSnapshotId(serverId string, volumeName string, snapshotName string) string {
	if serverId == "" {
		panic("serverId is empty")
	}
	if volumeName == "" {
		panic("serverId is empty")
	}
	if snapshotName == "" {
		panic("snapshotName is empty")
	}
	if strings.Contains(serverId, "/") {
		panic("serverId must not contain '/' symbol")
	}
	if strings.Contains(volumeName, "/") {
		panic("volumeName must not contain '/' symbol")
	}
	if strings.Contains(snapshotName, "/") {
		panic("snapshotName must not contain '/' symbol")
	}
	return snapshotPrefix + "/" + serverId + "/" + volumeName + "/" + snapshotName
}

func parseSnapshotId(id string) (serverId string, volumeName string, snapshotName string, err error) {
	parts := strings.Split(id, "/")
	if len(parts) != 4 {
		return "", "", "", fmt.Errorf("expected format 's/serverId/volumeName/snapshotName' but found %v slash-separated components: %v", len(parts), id)
	}
	if parts[0] != snapshotPrefix {
		return "", "", "", fmt.Errorf("expected format 's/serverId/volumeName/snapshotName' but id starts with different prefix: %v", id)
	}
	serverId = parts[1]
	volumeName = parts[2]
	snapshotName = parts[3]
	if serverId == "" {
		return "", "", "", fmt.Errorf("expected format 'v/serverId/volumeName/snapshotName' but serverId is empty: %v", id)
	}
	if volumeName == "" {
		return "", "", "", fmt.Errorf("expected format 'v/serverId/volumeName/snapshotName' but volumeName is empty: %v", id)
	}
	if snapshotName == "" {
		return "", "", "", fmt.Errorf("expected format 'v/serverId/volumeName/snapshotName' but snapshotName is empty: %v", id)
	}
	return
}
