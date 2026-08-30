package seed_test

import (
	"context"
	"testing"

	"github.com/MajestaNet/ide/internal/db"
	"github.com/MajestaNet/ide/internal/metadata"
)

// purgeOptionalDomainMetadata removes optional module metadata left by prior
// tests on a shared DATABASE_URL. Keep in sync as new optional modules ship.
func purgeOptionalDomainMetadata(t *testing.T, ctx context.Context, pool *db.Pool) {
	t.Helper()
	objs := []string{
		"Note",
		"MarketingListMember", "MarketingList", "Campaign", "Lead",
		"Email", "PhoneCall", "Appointment", "Task",
		"Address",
		"Competitor",
		"QuoteLine", "Quote", "OpportunityContactRole", "Opportunity",
		"OrderLine", "Order",
		"CaseComment", "WorkOrder", "ContractLineItem", "Entitlement", "Asset", "ServiceContract", "Case",
		"PriceListEntry", "PriceList", "Product", "Unit", "UnitGroup",
		// industry managed packs
		"MedicationRequest", "Observation", "AllergyIntolerance", "Condition", "Encounter", "CarePlan", "Practitioner", "Patient",
		"KYC", "MortgageApplication", "Limit", "Coverage", "Claim", "Collateral", "FinancialProduct", "Branch", "Bank",
		"SurveyResponse", "SurveyDefinition", "RetailAppointment", "ProductCategory", "ProductBrand", "CustomerAsset", "LoyaltyCard", "LoyaltyAccount", "LoyaltyProgram",
		"EmployeeCommuting", "BusinessTravel", "FuelType", "Material", "Emission", "EmissionFactor", "EmissionsSource", "Facility",
		"TestScore", "Internship", "Scholarship", "PreviousEducation", "CourseSection", "Course", "Program", "AreaOfStudy", "AcademicPeriod",
		"DeviceInspection", "DealDevice", "DealCustomer", "Deal", "BusinessFacility", "Device", "DeviceModel", "DeviceBrand",
		"Budget", "Indicator", "DeliveryFramework", "BenefitRecipient", "Disbursement", "Award", "DonorCommitment", "Designation",
		"CustomerJourney", "EventRegistration", "AttendeePass", "EventVendor", "Hotel", "Building", "MarketingEvent",
		"Poll", "Idea", "BlogPost", "Blog", "ForumPost", "ForumThread", "Forum", "Invitation", "WebRole", "WebPage", "Website",
		"Estimate", "Expense", "TimeEntry", "BookableResource", "Characteristic", "ProjectTask", "Project",
	}
	for _, api := range objs {
		_, _ = pool.Exec(ctx, `DELETE FROM metadata_relationships WHERE from_object=$1 OR to_object=$1`, api)
		_, _ = pool.Exec(ctx, `DELETE FROM metadata_fields WHERE object_api_name=$1`, api)
		_, _ = pool.Exec(ctx, `DELETE FROM object_permissions WHERE object_api_name=$1`, api)
		_, _ = pool.Exec(ctx, `DELETE FROM metadata_objects WHERE api_name=$1`, api)
		_, _ = pool.Exec(ctx, `DELETE FROM records WHERE object_api_name=$1`, api)
		_, _ = pool.Exec(ctx, `DELETE FROM records_hv WHERE object_api_name=$1`, api)
		_, _ = pool.Exec(ctx, `DELETE FROM field_projections WHERE object_api_name=$1`, api)
	}
	_, _ = pool.Exec(ctx, `DELETE FROM metadata_fields WHERE package_name = 'crm_bridge'`)
	_, _ = pool.Exec(ctx, `DELETE FROM package_installs WHERE package_name = ANY($1::text[])`,
		[]string{"notes", "catalog", "sales", "service", "crm_bridge", "billing", "agents_starter",
			"address", "activities", "lead_marketing",
			"healthcare", "financial_services", "retail", "sustainability", "education",
			"automotive", "nonprofit", "marketing_events", "portals", "project_service"})
}

func purgeAndInvalidate(t *testing.T, ctx context.Context, pool *db.Pool, meta *metadata.Service) {
	t.Helper()
	purgeOptionalDomainMetadata(t, ctx, pool)
	meta.InvalidateCache()
}
