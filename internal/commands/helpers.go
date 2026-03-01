package commands

func resolveTarget(url, cloudID string) string {
	if url != "" {
		return url
	}
	return cloudID
}
