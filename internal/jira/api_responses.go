package jira

// APIResponseIssueGet represents a Jira GET /rest/api/3/issue/{issueIdOrKey} response.
type APIResponseIssueGet struct {
	Fields APIResponseIssueGetFields `json:"fields"`
	Key    string                    `json:"key"`
}

// APIResponseIssueGetFields holds Jira issue fields.
type APIResponseIssueGetFields struct {
	Assignee  APIResponseIssueGetFieldsAssignee  `json:"assignee"`
	IssueType APIResponseIssueGetFieldsIssueType `json:"issuetype"`
	Priority  APIResponseIssueGetFieldsPriority  `json:"priority"`
	Project   APIResponseIssueGetFieldsPriority  `json:"project"`
	Reporter  APIResponseIssueGetFieldsReporter  `json:"reporter"`
	Status    APIResponseIssueGetFieldsStatus    `json:"status"`
	Summary   string                             `json:"summary"`
}

// APIResponseIssueGetFieldsAssignee holds Jira issue assignee info.
type APIResponseIssueGetFieldsAssignee struct {
	AvatarURLs   APIResponseIssueGetFieldsReporterAvatarURLs `json:"avatarUrls"`
	DisplayName  string                                      `json:"displayName"`
	EmailAddress string                                      `json:"emailAddress"`
}

// APIResponseIssueGetFieldsIssueType holds Jira issue type info.
type APIResponseIssueGetFieldsIssueType struct {
	IconURL string `json:"iconUrl"`
	Name    string `json:"name"`
}

// APIResponseIssueGetFieldsPriority holds Jira issue priority info.
type APIResponseIssueGetFieldsPriority struct {
	Name    string `json:"name"`
	IconURL string `json:"iconUrl"`
}

// APIResponseIssueGetFieldsProject holds Jira issue project info.
type APIResponseIssueGetFieldsProject struct {
	Name string `json:"name"`
}

// APIResponseIssueGetFieldsReporter holds Jira issue reporter info.
type APIResponseIssueGetFieldsReporter struct {
	AvatarURLs   APIResponseIssueGetFieldsReporterAvatarURLs `json:"avatarUrls"`
	DisplayName  string                                      `json:"displayName"`
	EmailAddress string                                      `json:"emailAddress"`
}

// APIResponseIssueGetFieldsReporterAvatarURLs holds Jira avatar URLs.
type APIResponseIssueGetFieldsReporterAvatarURLs struct {
	FourtyEightByFourtyEight string `json:"48x48"`
}

// APIResponseIssueGetFieldsStatus holds Jira issue status info.
type APIResponseIssueGetFieldsStatus struct {
	Name string `json:"name"`
}

// APIResponseIssuePickerGet represents a Jira GET /rest/api/3/issue/picker response.
type APIResponseIssuePickerGet struct {
	Sections []APIResponseIssuePickerGetSection `json:"sections"`
}

// APIResponseIssuePickerGetSection holds Jira issue picker section data.
type APIResponseIssuePickerGetSection struct {
	ID     string                                  `json:"id"`
	Label  string                                  `json:"label"`
	Issues []APIResponseIssuePickerGetSectionIssue `json:"issues"`
}

// APIResponseIssuePickerGetSectionIssue holds Jira issue picker issue data.
type APIResponseIssuePickerGetSectionIssue struct {
	Key         string `json:"key"`
	Img         string `json:"img"`
	SummaryText string `json:"summaryText"`
	ID          int    `json:"id"`
}
