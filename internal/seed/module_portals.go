package seed

import "github.com/MajestaNet/ide/internal/packages"

const PortalsPackageVersion = "1.0.0"

func registerPortalsModule() {
	packages.Register(packages.Module{
		Name:              "portals",
		Version:           PortalsPackageVersion,
		Label:             "Portals",
		Description:       "Portal surfaces: website, pages, forums, blogs, ideas, polls",
		DependsOn:         []string{"core"},
		Optional:          true,
		DocumentationPath: "docs/modules/portals.md",
		Objects: []packages.ObjectDef{
			{
				APIName: "Website", Label: "Website", PluralLabel: "Websites",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					textField("PrimaryDomain", "Primary Domain", 255, true),
					statusField(),
					descriptionField(),
				},
			},
			{
				APIName: "WebPage", Label: "Web Page", PluralLabel: "Web Pages",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					textField("PartialURL", "Partial URL", 255, true),
					lookupField("WebsiteId", "Website", "Website", "WebPages", true),
					lookupField("ParentPageId", "Parent Page", "WebPage", "ChildPages", false),
					statusField("Draft", "Published", "Archived"),
					descriptionField(),
				},
			},
			{
				APIName: "WebRole", Label: "Web Role", PluralLabel: "Web Roles",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					lookupField("WebsiteId", "Website", "Website", "WebRoles", true),
					boolField("AuthenticatedUsersRole", "Authenticated Users Role"),
					boolField("AnonymousUsersRole", "Anonymous Users Role"),
					descriptionField(),
				},
			},
			{
				APIName: "Invitation", Label: "Invitation", PluralLabel: "Invitations",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					textField("InviteCode", "Invite Code", 100, true),
					lookupField("WebsiteId", "Website", "Website", "Invitations", true),
					contactLookup("Invitations"),
					statusField("Sent", "Accepted", "Expired", "Revoked"),
					dateField("ExpiryDate", "Expiry Date"),
				},
			},
			{
				APIName: "Forum", Label: "Forum", PluralLabel: "Forums",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					lookupField("WebsiteId", "Website", "Website", "Forums", true),
					statusField(),
					descriptionField(),
				},
			},
			{
				APIName: "ForumThread", Label: "Forum Thread", PluralLabel: "Forum Threads",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					lookupField("ForumId", "Forum", "Forum", "ForumThreads", true),
					contactLookup("ForumThreads"),
					boolField("IsSticky", "Sticky"),
					statusField("Open", "Closed"),
				},
			},
			{
				APIName: "ForumPost", Label: "Forum Post", PluralLabel: "Forum Posts",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					lookupField("ForumThreadId", "Forum Thread", "ForumThread", "ForumPosts", true),
					contactLookup("ForumPosts"),
					descriptionField(),
				},
			},
			{
				APIName: "Blog", Label: "Blog", PluralLabel: "Blogs",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					lookupField("WebsiteId", "Website", "Website", "Blogs", true),
					statusField(),
					descriptionField(),
				},
			},
			{
				APIName: "BlogPost", Label: "Blog Post", PluralLabel: "Blog Posts",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					lookupField("BlogId", "Blog", "Blog", "BlogPosts", true),
					dateField("PublishDate", "Publish Date"),
					statusField("Draft", "Published", "Archived"),
					descriptionField(),
				},
			},
			{
				APIName: "Idea", Label: "Idea", PluralLabel: "Ideas",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					lookupField("WebsiteId", "Website", "Website", "Ideas", true),
					contactLookup("Ideas"),
					numberField("VoteCount", "Vote Count"),
					statusField("New", "Under Review", "Accepted", "Completed", "Rejected"),
					descriptionField(),
				},
			},
			{
				APIName: "Poll", Label: "Poll", PluralLabel: "Polls",
				Features: map[string]bool{"history": true},
				Fields: []packages.FieldDef{
					nameRequiredField(),
					lookupField("WebsiteId", "Website", "Website", "Polls", true),
					statusField("Draft", "Open", "Closed"),
					descriptionField(),
				},
			},
		},
	})
}
