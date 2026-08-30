package actions

import (
	"context"
	"strings"
	"time"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/dataengine"
)

func convertLead(ctx context.Context, s *Service, actor *authz.Actor, input map[string]any) (map[string]any, error) {
	leadID := strings.TrimSpace(strVal(input["leadId"]))
	if leadID == "" {
		return nil, errValidation("leadId is required")
	}
	createOpp := boolVal(input["createOpportunity"])
	if createOpp {
		if err := s.requirePackage(ctx, "sales", "createOpportunity"); err != nil {
			return nil, err
		}
	}
	convertedStatus := strings.TrimSpace(strVal(input["convertedStatus"]))
	if convertedStatus != "" && convertedStatus != "Converted" {
		return nil, errValidation("convertedStatus v1 only supports Converted")
	}

	if err := s.assertObject(ctx, actor, "Lead", authz.ActionRead); err != nil {
		return nil, err
	}
	lead, err := s.Data.Get(ctx, "Lead", leadID)
	if err != nil {
		return nil, err
	}
	if err := s.assertViewRecord(ctx, actor, lead, "Lead"); err != nil {
		return nil, err
	}
	if err := s.assertReadableFields(ctx, actor, "Lead",
		"Status", "AccountId", "ContactId", "Company", "FirstName", "LastName", "Email", "Phone", "Description", "Source",
	); err != nil {
		return nil, err
	}

	status := strings.TrimSpace(strVal(lead["Status"]))
	existingAccountID := strings.TrimSpace(strVal(lead["AccountId"]))
	existingContactID := strings.TrimSpace(strVal(lead["ContactId"]))
	if status == "Converted" && existingAccountID != "" && existingContactID != "" {
		return map[string]any{
			"leadId":           leadID,
			"accountId":        existingAccountID,
			"contactId":        existingContactID,
			"alreadyConverted": true,
		}, nil
	}
	if status == "Converted" {
		return nil, errValidation("Lead is Converted but missing AccountId or ContactId")
	}

	accountID, err := s.resolveConvertAccount(ctx, actor, lead, strings.TrimSpace(strVal(input["accountId"])))
	if err != nil {
		return nil, err
	}
	contactID, err := s.resolveConvertContact(ctx, actor, lead, accountID, strings.TrimSpace(strVal(input["contactId"])))
	if err != nil {
		return nil, err
	}

	var opportunityID string
	if createOpp {
		opportunityID, err = s.createConvertOpportunity(ctx, actor, lead, accountID, contactID, input)
		if err != nil {
			return nil, err
		}
	}

	leadPatch := map[string]any{
		"Status":    "Converted",
		"AccountId": accountID,
		"ContactId": contactID,
	}
	if err := s.assertModifyRecord(ctx, actor, lead, "Lead"); err != nil {
		return nil, err
	}
	if err := s.assertEditable(ctx, actor, "Lead", leadPatch); err != nil {
		return nil, err
	}
	if err := dataengine.CountSyncMutation(ctx); err != nil {
		return nil, err
	}
	if _, err := s.Data.Update(ctx, "Lead", leadID, leadPatch, actor); err != nil {
		return nil, err
	}

	out := map[string]any{
		"leadId":           leadID,
		"accountId":        accountID,
		"contactId":        contactID,
		"alreadyConverted": false,
	}
	if opportunityID != "" {
		out["opportunityId"] = opportunityID
	}
	return out, nil
}

func (s *Service) resolveConvertAccount(ctx context.Context, actor *authz.Actor, lead map[string]any, accountID string) (string, error) {
	if accountID != "" {
		acct, err := s.Data.Get(ctx, "Account", accountID)
		if err != nil {
			return "", err
		}
		if err := s.assertViewRecord(ctx, actor, acct, "Account"); err != nil {
			return "", err
		}
		return accountID, nil
	}
	name := derivedAccountName(lead)
	if name == "" {
		return "", errValidation("Lead.Company (or a derived name) is required to create Account")
	}
	data := map[string]any{"Name": name}
	if phone := strings.TrimSpace(strVal(lead["Phone"])); phone != "" {
		data["Phone"] = phone
	}
	if desc := strings.TrimSpace(strVal(lead["Description"])); desc != "" {
		data["Description"] = desc
	}
	if err := s.assertObject(ctx, actor, "Account", authz.ActionCreate); err != nil {
		return "", err
	}
	if err := s.assertEditable(ctx, actor, "Account", data); err != nil {
		return "", err
	}
	if err := dataengine.CountSyncMutation(ctx); err != nil {
		return "", err
	}
	rec, err := s.Data.Create(ctx, "Account", data, actor)
	if err != nil {
		return "", err
	}
	id, _ := rec["Id"].(string)
	return id, nil
}

func (s *Service) resolveConvertContact(ctx context.Context, actor *authz.Actor, lead map[string]any, accountID, contactID string) (string, error) {
	if contactID != "" {
		contact, err := s.Data.Get(ctx, "Contact", contactID)
		if err != nil {
			return "", err
		}
		if err := s.assertViewRecord(ctx, actor, contact, "Contact"); err != nil {
			return "", err
		}
		if strings.TrimSpace(strVal(contact["AccountId"])) == "" && accountID != "" {
			patch := map[string]any{"AccountId": accountID}
			if err := s.assertModifyRecord(ctx, actor, contact, "Contact"); err != nil {
				return "", err
			}
			if err := s.assertEditable(ctx, actor, "Contact", patch); err != nil {
				return "", err
			}
			if err := dataengine.CountSyncMutation(ctx); err != nil {
				return "", err
			}
			if _, err := s.Data.Update(ctx, "Contact", contactID, patch, actor); err != nil {
				return "", err
			}
		}
		return contactID, nil
	}
	lastName := strings.TrimSpace(strVal(lead["LastName"]))
	if lastName == "" {
		return "", errValidation("Lead.LastName is required to create Contact")
	}
	data := map[string]any{"LastName": lastName}
	if fn := strings.TrimSpace(strVal(lead["FirstName"])); fn != "" {
		data["FirstName"] = fn
	}
	if email := strings.TrimSpace(strVal(lead["Email"])); email != "" {
		data["Email"] = email
	}
	if phone := strings.TrimSpace(strVal(lead["Phone"])); phone != "" {
		data["MobilePhone"] = phone
	}
	if accountID != "" {
		data["AccountId"] = accountID
	}
	if err := s.assertObject(ctx, actor, "Contact", authz.ActionCreate); err != nil {
		return "", err
	}
	if err := s.assertEditable(ctx, actor, "Contact", data); err != nil {
		return "", err
	}
	if err := dataengine.CountSyncMutation(ctx); err != nil {
		return "", err
	}
	rec, err := s.Data.Create(ctx, "Contact", data, actor)
	if err != nil {
		return "", err
	}
	id, _ := rec["Id"].(string)
	return id, nil
}

func (s *Service) createConvertOpportunity(ctx context.Context, actor *authz.Actor, lead map[string]any, accountID, contactID string, input map[string]any) (string, error) {
	name := strings.TrimSpace(strVal(input["opportunityName"]))
	if name == "" {
		name = derivedAccountName(lead)
	}
	if name == "" {
		name = strings.TrimSpace(strVal(lead["LastName"]))
	}
	if name == "" {
		return "", errValidation("opportunityName is required when creating Opportunity")
	}
	closeDate := strings.TrimSpace(strVal(input["opportunityCloseDate"]))
	if closeDate == "" {
		closeDate = time.Now().UTC().AddDate(0, 0, 30).Format("2006-01-02")
	}
	data := map[string]any{
		"Name":      name,
		"StageName": "Prospecting",
		"CloseDate": closeDate,
	}
	if accountID != "" {
		data["AccountId"] = accountID
	}
	if contactID != "" {
		data["ContactId"] = contactID
	}
	if accountID == "" && contactID == "" {
		return "", errValidation("Opportunity requires AccountId or ContactId")
	}
	if src := strings.TrimSpace(strVal(lead["Source"])); src != "" {
		data["LeadSource"] = src
	}
	if err := s.assertObject(ctx, actor, "Opportunity", authz.ActionCreate); err != nil {
		return "", err
	}
	if err := s.assertEditable(ctx, actor, "Opportunity", data); err != nil {
		return "", err
	}
	if err := dataengine.CountSyncMutation(ctx); err != nil {
		return "", err
	}
	rec, err := s.Data.Create(ctx, "Opportunity", data, actor)
	if err != nil {
		return "", err
	}
	id, _ := rec["Id"].(string)
	return id, nil
}

func derivedAccountName(lead map[string]any) string {
	if company := strings.TrimSpace(strVal(lead["Company"])); company != "" {
		return company
	}
	fn := strings.TrimSpace(strVal(lead["FirstName"]))
	ln := strings.TrimSpace(strVal(lead["LastName"]))
	switch {
	case fn != "" && ln != "":
		return fn + " " + ln
	case ln != "":
		return ln
	default:
		return fn
	}
}

func strVal(v any) string {
	switch t := v.(type) {
	case string:
		return t
	default:
		return ""
	}
}

func boolVal(v any) bool {
	b, _ := v.(bool)
	return b
}
