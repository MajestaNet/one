package seed

import "github.com/MajestaNet/ide/internal/packages"

const EducationPackageVersion = "1.0.0"

func registerEducationModule() {
	packages.Register(packages.Module{
		Name:              "education",
		Version:           EducationPackageVersion,
		Label:             "Education",
		Description:       "Education: programs, courses, scholarships, internships",
		DependsOn:         []string{"core"},
		Optional:          true,
		DocumentationPath: "docs/modules/education.md",
		Objects: []packages.ObjectDef{
			{
				APIName: "AcademicPeriod", Label: "Academic Period", PluralLabel: "Academic Periods",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					dateField("StartDate", "Start Date"),
					dateField("EndDate", "End Date"),
					statusField(),
					descriptionField(),
				},
			},
			{
				APIName: "AreaOfStudy", Label: "Area of Study", PluralLabel: "Areas of Study",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					textField("Code", "Code", 50, true),
					statusField(),
					descriptionField(),
				},
			},
			{
				APIName: "Program", Label: "Program", PluralLabel: "Programs",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					textField("ProgramCode", "Program Code", 50, true),
					lookupField("AreaOfStudyId", "Area of Study", "AreaOfStudy", "Programs", false),
					statusField(),
					descriptionField(),
				},
			},
			{
				APIName: "Course", Label: "Course", PluralLabel: "Courses",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					textField("CourseCode", "Course Code", 50, true),
					numberField("Credits", "Credits"),
					lookupField("ProgramId", "Program", "Program", "Courses", false),
					statusField(),
					descriptionField(),
				},
			},
			{
				APIName: "CourseSection", Label: "Course Section", PluralLabel: "Course Sections",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					textField("SectionNumber", "Section Number", 50, true),
					lookupField("CourseId", "Course", "Course", "CourseSections", true),
					lookupField("AcademicPeriodId", "Academic Period", "AcademicPeriod", "CourseSections", false),
					numberField("Capacity", "Capacity"),
					statusField("Open", "Closed", "Cancelled"),
				},
			},
			{
				APIName: "PreviousEducation", Label: "Previous Education", PluralLabel: "Previous Education",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					textField("Institution", "Institution", 255, true),
					textField("Degree", "Degree", 100, false),
					dateField("GraduationDate", "Graduation Date"),
					contactLookup("PreviousEducation"),
					descriptionField(),
				},
			},
			{
				APIName: "Scholarship", Label: "Scholarship", PluralLabel: "Scholarships",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					currencyField("Amount", "Amount"),
					statusField("Open", "Awarded", "Closed"),
					dateField("ApplicationDeadline", "Application Deadline"),
					descriptionField(),
				},
			},
			{
				APIName: "Internship", Label: "Internship", PluralLabel: "Internships",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					statusField("Open", "Filled", "Completed", "Cancelled"),
					dateField("StartDate", "Start Date"),
					dateField("EndDate", "End Date"),
					accountLookup("Internships"),
					contactLookup("Internships"),
					descriptionField(),
				},
			},
			{
				APIName: "TestScore", Label: "Test Score", PluralLabel: "Test Scores",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					textField("TestType", "Test Type", 100, true),
					numberField("Score", "Score"),
					dateField("TestDate", "Test Date"),
					contactLookup("TestScores"),
					descriptionField(),
				},
			},
		},
	})
}
