package memories

import "embed"

// all:skills embeds the entire skills tree (recursively, including any nested
// skill directories). The "skills/**/SKILL.md" glob looked recursive but was
// not — Go's ** matches exactly one path component.
//
//go:embed all:skills
var Skills embed.FS
