package automation

// FrozenSDKMethods is the guest AutomationContext surface (ADR-014).
// http/connector are async-only (BP-014); sync hosts reject them at runtime.
var FrozenSDKMethods = []string{
	"getRecord",
	"createRecord",
	"updateRecord",
	"deleteRecord",
	"query",
	"log",
	"invokeAction",
	"http",
	"connector",
}

// FrozenUnitHelpers are available only in the unit harness (tests/automations/**).
var FrozenUnitHelpers = []string{
	"runUnderTest",
	"getCalls",
	"clearCalls",
}

// IsFrozenSDKMethod reports whether name is part of the v1 production SDK.
func IsFrozenSDKMethod(name string) bool {
	for _, m := range FrozenSDKMethods {
		if m == name {
			return true
		}
	}
	return false
}
