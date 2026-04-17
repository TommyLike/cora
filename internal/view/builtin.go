package view

// builtinViews contains the default display configurations shipped with cora.
// User entries in views.yaml override individual keys here (whole-config replacement,
// not column merging).
var builtinViews = map[string]map[string]ViewConfig{
	"gitcode": {
		"issues/get": {
			Columns: []ViewColumn{
				{Field: "number", Label: "No."},
				{Field: "title", Label: "Title", Truncate: 120},
				{Field: "state", Label: "State"},
				{Field: "user.login", Label: "Author"},
				{Field: "assignees", Label: "Assignees", Format: FormatJSON},
				{Field: "labels", Label: "Labels", Format: FormatJSON},
				{Field: "created_at", Label: "Created", Format: FormatDate},
				{Field: "updated_at", Label: "Updated", Format: FormatDate},
				{Field: "body", Label: "Description", Format: FormatMultiline, Truncate: 600},
			},
		},
		"issues/list": {
			Columns: []ViewColumn{
				{Field: "number", Label: "No.", Width: 6},
				{Field: "title", Label: "Title", Truncate: 50, Width: 52},
				{Field: "state", Label: "State", Width: 8},
				{Field: "user.login", Label: "Author", Width: 18},
				{Field: "created_at", Label: "Created", Format: FormatDate, Width: 12},
			},
		},
		"repos/get": {
			Columns: []ViewColumn{
				{Field: "full_name", Label: "Repo"},
				{Field: "description", Label: "Description", Truncate: 80},
				{Field: "stargazers_count", Label: "Stars"},
				{Field: "forks_count", Label: "Forks"},
				{Field: "language", Label: "Language"},
				{Field: "license.name", Label: "License"},
				{Field: "topics", Label: "Topics", Format: FormatJSON},
				{Field: "created_at", Label: "Created", Format: FormatDate},
			},
		},
		"repos/list": {
			Columns: []ViewColumn{
				{Field: "full_name", Label: "Repo", Width: 32},
				{Field: "description", Label: "Description", Truncate: 40, Width: 42},
				{Field: "stargazers_count", Label: "Stars", Width: 8},
				{Field: "language", Label: "Language", Width: 12},
			},
		},
		"pulls/list": {
			Columns: []ViewColumn{
				{Field: "number", Label: "No.", Width: 6},
				{Field: "title", Label: "Title", Truncate: 50, Width: 52},
				{Field: "state", Label: "State", Width: 8},
				{Field: "user.login", Label: "Author", Width: 18},
				{Field: "head.label", Label: "Branch", Width: 24},
				{Field: "created_at", Label: "Created", Format: FormatDate, Width: 12},
			},
		},
		"pulls/get": {
			Columns: []ViewColumn{
				{Field: "number", Label: "No."},
				{Field: "title", Label: "Title", Truncate: 120},
				{Field: "state", Label: "State"},
				{Field: "user.login", Label: "Author"},
				{Field: "head.label", Label: "From Branch"},
				{Field: "base.label", Label: "Into Branch"},
				{Field: "created_at", Label: "Created", Format: FormatDate},
				{Field: "body", Label: "Description", Format: FormatMultiline, Truncate: 600},
			},
		},
	},

	"forum": {
		"topics/list": {
			Columns: []ViewColumn{
				{Field: "id", Label: "ID", Width: 8},
				{Field: "title", Label: "Title", Truncate: 60, Width: 62},
				{Field: "posts_count", Label: "Posts", Width: 8},
				{Field: "reply_count", Label: "Replies", Width: 8},
				{Field: "created_at", Label: "Created", Format: FormatDate, Width: 12},
			},
		},
		"topics/get": {
			Columns: []ViewColumn{
				{Field: "id", Label: "ID"},
				{Field: "title", Label: "Title"},
				{Field: "posts_count", Label: "Posts"},
				{Field: "reply_count", Label: "Replies"},
				{Field: "created_at", Label: "Created", Format: FormatDate},
				{Field: "category_id", Label: "Category"},
			},
		},
		"posts/list": {
			Columns: []ViewColumn{
				{Field: "id", Label: "ID", Width: 8},
				{Field: "username", Label: "Author", Width: 18},
				{Field: "cooked", Label: "Content", Format: FormatMultiline, Truncate: 200, Width: 60},
				{Field: "created_at", Label: "Created", Format: FormatDate, Width: 12},
			},
		},
	},

	"etherpad": {
		"pads/list": {
			Columns: []ViewColumn{
				{Field: "padID", Label: "Pad ID"},
			},
		},
	},
}
