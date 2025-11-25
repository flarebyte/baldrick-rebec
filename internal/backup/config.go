package backup

// BackupEntityConfig describes a logical entity and how to back it up.
type BackupEntityConfig struct {
    EntityName       string   // Logical name (e.g., "projects").
    TableName        string   // Physical table (e.g., "projects").
    PKColumns        []string // Primary key columns (ordered).
    HasRoleName      bool     // True if the table contains a role_name column.
    IncludeByDefault bool     // Whether the entity is included by default in backup.
}

// DefaultEntities returns the seeded configuration for entities considered permanent-ish.
func DefaultEntities() []BackupEntityConfig {
    return []BackupEntityConfig{
        {EntityName: "roles", TableName: "roles", PKColumns: []string{"name"}, HasRoleName: false, IncludeByDefault: true},
        {EntityName: "workflows", TableName: "workflows", PKColumns: []string{"name"}, HasRoleName: true, IncludeByDefault: true},
        {EntityName: "tags", TableName: "tags", PKColumns: []string{"name"}, HasRoleName: true, IncludeByDefault: true},
        {EntityName: "projects", TableName: "projects", PKColumns: []string{"name", "role_name"}, HasRoleName: true, IncludeByDefault: true},
        {EntityName: "stores", TableName: "stores", PKColumns: []string{"id"}, HasRoleName: true, IncludeByDefault: true},
        {EntityName: "scripts", TableName: "scripts", PKColumns: []string{"id"}, HasRoleName: true, IncludeByDefault: true},
        {EntityName: "tasks", TableName: "tasks", PKColumns: []string{"id"}, HasRoleName: true, IncludeByDefault: true},
        {EntityName: "topics", TableName: "topics", PKColumns: []string{"name", "role_name"}, HasRoleName: true, IncludeByDefault: true},
        {EntityName: "workspaces", TableName: "workspaces", PKColumns: []string{"id"}, HasRoleName: true, IncludeByDefault: true},
        {EntityName: "blackboards", TableName: "blackboards", PKColumns: []string{"id"}, HasRoleName: true, IncludeByDefault: true},
        {EntityName: "stickies", TableName: "stickies", PKColumns: []string{"id"}, HasRoleName: false, IncludeByDefault: true},
        {EntityName: "stickie_relations", TableName: "stickie_relations", PKColumns: []string{"from_id", "to_id", "rel_type"}, HasRoleName: false, IncludeByDefault: true},
        {EntityName: "task_replaces", TableName: "task_replaces", PKColumns: []string{"new_task_id", "old_task_id"}, HasRoleName: false, IncludeByDefault: true},
        {EntityName: "packages", TableName: "packages", PKColumns: []string{"id"}, HasRoleName: true, IncludeByDefault: true},
        // Ephemeral by default (can be opted-in via --include)
        {EntityName: "conversations", TableName: "conversations", PKColumns: []string{"id"}, HasRoleName: true, IncludeByDefault: false},
        {EntityName: "experiments", TableName: "experiments", PKColumns: []string{"id"}, HasRoleName: false, IncludeByDefault: false},
        {EntityName: "messages", TableName: "messages", PKColumns: []string{"id"}, HasRoleName: true, IncludeByDefault: false},
        {EntityName: "messages_content", TableName: "messages_content", PKColumns: []string{"id"}, HasRoleName: false, IncludeByDefault: false},
        {EntityName: "queues", TableName: "queues", PKColumns: []string{"id"}, HasRoleName: false, IncludeByDefault: false},
        {EntityName: "testcases", TableName: "testcases", PKColumns: []string{"id"}, HasRoleName: true, IncludeByDefault: false},
        {EntityName: "task_variants", TableName: "task_variants", PKColumns: []string{"variant"}, HasRoleName: false, IncludeByDefault: true},
        {EntityName: "scripts_content", TableName: "scripts_content", PKColumns: []string{"id"}, HasRoleName: false, IncludeByDefault: true},
    }
}

