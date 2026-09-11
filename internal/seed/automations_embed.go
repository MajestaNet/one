package seed

import _ "embed"

//go:embed automations/Lead_ConvertOnConvertedStatus.ts
var leadConvertOnConvertedStatusSrc string

//go:embed automations/Quote_AcceptOnStatusAccepted.ts
var quoteAcceptOnStatusAcceptedSrc string
