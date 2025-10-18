package main

import "strings"

type ParsedQuery struct {
	Query      string
	Directives []Directive
}

type Directive struct {
	name   string
	params []string
}

func ParseDirective(line string) Directive {
	line = strings.TrimSpace(line[2:])

	if !strings.HasPrefix(line, "@wtf-") {
		return Directive{}
	}

	line = strings.TrimPrefix(line, "@wtf-")

	parts := strings.Fields(line)
	if len(parts) == 0 {
		return Directive{}
	}

	directive := Directive{
		name: parts[0],
	}

	if len(parts) > 1 {
		directive.params = parts[1:]
	}

	return directive
}

func ParseQueries(sqlBlob string) []ParsedQuery {
	queries := strings.Split(sqlBlob, ";")

	var parsedQueries []ParsedQuery

	for _, query := range queries {
		query = strings.TrimSpace(query)
		if query == "" {
			continue
		}

		parsedQuery := ParsedQuery{
			Query:      "",
			Directives: []Directive{},
		}

		lines := strings.Split(query, "\n")
		var queryLines []string

		for _, line := range lines {
			trimmedLine := strings.TrimSpace(line)

			// Skip empty lines
			if trimmedLine == "" {
				continue
			}

			// Check if it's a comment line that might contain a directive
			if strings.HasPrefix(trimmedLine, "--") {
				directive := ParseDirective(trimmedLine)
				if directive.name != "" {
					parsedQuery.Directives = append(parsedQuery.Directives, directive)
				} else {
					// It's a regular comment, keep it as part of the query
					queryLines = append(queryLines, line)
				}
			} else {
				// It's a regular SQL line
				queryLines = append(queryLines, line)
			}
		}

		// Reconstruct the query from collected lines
		parsedQuery.Query = strings.Join(queryLines, "\n")

		// Only add non-empty queries
		if strings.TrimSpace(parsedQuery.Query) != "" {
			parsedQueries = append(parsedQueries, parsedQuery)
		}
	}

	return parsedQueries
}
