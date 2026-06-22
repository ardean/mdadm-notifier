package mdadm

// ReadSyncProgress reads rebuild/resync progress from /proc/mdstat only.
// detail is optional and used to locate the array when the device name differs.
func ReadSyncProgress(device, detail string) (*SyncProgress, error) {
	mdstat, err := ReadMdstat()
	if err != nil {
		return nil, err
	}

	if progress, ok := ParseSyncProgressWithDetail(device, mdstat, detail); ok {
		return &progress, nil
	}

	return &SyncProgress{Active: false}, nil
}
