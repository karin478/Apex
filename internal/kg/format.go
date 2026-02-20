package kg

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ---------------------------------------------------------------------------
// FormatEntitiesTable
// ---------------------------------------------------------------------------

// FormatEntitiesTable renders a slice of entities as an aligned table with
// columns: TYPE | NAME | PROJECT | NAMESPACE. Returns "No entities found."
// when the input is empty or nil.
func FormatEntitiesTable(entities []*Entity) string {
	if len(entities) == 0 {
		return "No entities found."
	}

	// Determine column widths using header labels as minimums.
	typeW := len("TYPE")
	nameW := len("NAME")
	projW := len("PROJECT")
	nsW := len("NAMESPACE")

	for _, e := range entities {
		if l := len(string(e.Type)); l > typeW {
			typeW = l
		}
		if l := len(e.CanonicalName); l > nameW {
			nameW = l
		}
		if l := len(e.Project); l > projW {
			projW = l
		}
		if l := len(e.Namespace); l > nsW {
			nsW = l
		}
	}

	var b strings.Builder

	// Header row.
	rowFmt := fmt.Sprintf("%%-%ds | %%-%ds | %%-%ds | %%s\n", typeW, nameW, projW)
	fmt.Fprintf(&b, rowFmt, "TYPE", "NAME", "PROJECT", "NAMESPACE")

	// Separator line.
	lineLen := typeW + 3 + nameW + 3 + projW + 3 + nsW
	b.WriteString(strings.Repeat("-", lineLen))
	b.WriteString("\n")

	// Data rows.
	for _, e := range entities {
		fmt.Fprintf(&b, rowFmt, string(e.Type), e.CanonicalName, e.Project, e.Namespace)
	}

	return b.String()
}

// ---------------------------------------------------------------------------
// FormatQueryResult
// ---------------------------------------------------------------------------

// FormatQueryResult renders a center entity with its related entities and
// relationships in a human-readable detail view. The output shows the center
// entity header, then lists related entities annotated with their relationship
// type.
func FormatQueryResult(center *Entity, entities []*Entity, rels []*Relationship) string {
	var b strings.Builder

	// Center entity header.
	fmt.Fprintf(&b, "Entity: %s (%s)\n", center.CanonicalName, string(center.Type))
	fmt.Fprintf(&b, "  Project:   %s\n", center.Project)
	fmt.Fprintf(&b, "  Namespace: %s\n", center.Namespace)
	fmt.Fprintf(&b, "  ID:        %s\n", center.ID)

	// Build lookup: entity ID -> entity for fast access.
	entByID := make(map[string]*Entity, len(entities)+1)
	entByID[center.ID] = center
	for _, e := range entities {
		entByID[e.ID] = e
	}

	// Build lookup: for each related entity, find the relationship connecting
	// it to the center (or to the traversal path). We pair each related entity
	// with the rel_type of the relationship that discovered it.
	type relatedInfo struct {
		entity  *Entity
		relType RelType
	}

	var related []relatedInfo
	for _, e := range entities {
		// Find the relationship that connects this entity.
		var rt RelType
		for _, r := range rels {
			if (r.FromID == center.ID && r.ToID == e.ID) ||
				(r.ToID == center.ID && r.FromID == e.ID) {
				rt = r.RelType
				break
			}
		}
		related = append(related, relatedInfo{entity: e, relType: rt})
	}

	fmt.Fprintf(&b, "\nRelated (%d entities):\n", len(entities))
	for _, ri := range related {
		fmt.Fprintf(&b, "  %s -> %s (%s)\n", string(ri.relType), ri.entity.CanonicalName, string(ri.entity.Type))
	}

	return b.String()
}

// ---------------------------------------------------------------------------
// FormatJSON
// ---------------------------------------------------------------------------

// FormatJSON returns the given value serialised as indented JSON with 2-space
// indentation. If marshalling fails it returns the error string.
func FormatJSON(v interface{}) string {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("json error: %v", err)
	}
	return string(data)
}
