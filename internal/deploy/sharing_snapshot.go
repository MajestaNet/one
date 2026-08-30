package deploy

// sharingSnapshotFromArtifact builds a snapshot map for sharing apply from bundle fields.
func sharingSnapshotFromArtifact(artifact *BundleArtifact) map[string]any {
	if artifact == nil {
		return nil
	}
	out := map[string]any{}
	if len(artifact.DataRoles) > 0 {
		raw := make([]any, 0, len(artifact.DataRoles))
		for _, r := range artifact.DataRoles {
			m := map[string]any{"apiName": r.APIName, "label": r.Label}
			if r.ParentDataRoleAPIName != "" {
				m["parentDataRoleApiName"] = r.ParentDataRoleAPIName
			}
			raw = append(raw, m)
		}
		out["dataRoles"] = raw
	}
	if len(artifact.ObjectSharingSettings) > 0 {
		raw := make([]any, 0, len(artifact.ObjectSharingSettings))
		for _, o := range artifact.ObjectSharingSettings {
			raw = append(raw, map[string]any{
				"objectApiName":       o.ObjectAPIName,
				"defaultAccess":       o.DefaultAccess,
				"sharingRulesEnabled": o.SharingRulesEnabled,
			})
		}
		out["objectSharingSettings"] = raw
	}
	if len(artifact.SharingRules) > 0 {
		raw := make([]any, 0, len(artifact.SharingRules))
		for _, r := range artifact.SharingRules {
			raw = append(raw, map[string]any{
				"objectApiName":           r.ObjectAPIName,
				"apiName":                 r.APIName,
				"label":                   r.Label,
				"active":                  r.Active,
				"accessLevel":             r.AccessLevel,
				"sharedToDataRoleApiName": r.SharedToDataRoleAPIName,
				"criteria":                r.Criteria,
				"sortOrder":               r.SortOrder,
			})
		}
		out["sharingRules"] = raw
	}
	return out
}
