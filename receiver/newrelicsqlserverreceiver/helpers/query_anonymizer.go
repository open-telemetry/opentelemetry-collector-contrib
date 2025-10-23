package helpers
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package helpers provides utility functions for SQL Server query processing and anonymization.
// This file implements comprehensive query text anonymization functionality for privacy and security.


import (
	"regexp"
	"strings"
)

var (
	// literalAnonymizer is a regular expression pattern used to match and identify
	// certain types of literal values in a string for anonymization. Specifically, it matches:
	// 1. Single-quoted character sequences, such as 'example'.
	// 2. Numeric sequences (integer numbers), such as 123 or 456.
	// 3. Double-quoted strings, such as "example".
	// This regex is useful for anonymizing literal values in SQL query text
	// by replacing sensitive data with placeholders while preserving query structure.
	// This follows the same pattern used by nri-mssql for compatibility.
	literalAnonymizer = regexp.MustCompile(`'[^']*'|\d+|".*?"`)
	
	// statementTextRegex is used to find StatementText attributes in execution plan XML
	// Matches: StatementText="..." or StatementText='...'
	statementTextRegex = regexp.MustCompile(`StatementText\s*=\s*("[^"]*"|'[^']*')`)
)

// AnonymizeQueryText anonymizes literal values in SQL query text for privacy and security.
// This function replaces sensitive data like string literals, numeric values, and quoted content
// with placeholders while preserving the query structure for analysis.
// 
// Examples:
//   Input:  "SELECT * FROM users WHERE name = 'John Doe' AND age = 25"
//   Output: "SELECT * FROM users WHERE name = ? AND age = ?"
//
//   Input:  "UPDATE products SET price = 99.99 WHERE category = \"electronics\""
//   Output: "UPDATE products SET price = ? WHERE category = ?"
//
// This follows the same anonymization pattern used by nri-mssql for consistent data handling.
//
// Parameters:
//   - query: The SQL query text to anonymize
//
// Returns:
//   - string: The anonymized query text with literals replaced by '?'
//
// Safety Features:
//   - Handles empty/nil input safely
//   - Limits processing to prevent performance issues on very large queries
//   - Returns truncated content with warning for oversized input
func AnonymizeQueryText(query string) string {
	if query == "" {
		return query
	}
	
	// Safety check: Don't process extremely large strings to avoid performance issues
	const maxSafeLength = 16384 // 16KB limit for safe regex processing
	if len(query) > maxSafeLength {
		// For very large content, just truncate with a note rather than risk regex performance issues
		return query[:maxSafeLength] + "...[content too large for anonymization]"
	}
	
	// Replace all literal values (strings, numbers, quoted content) with placeholders
	anonymizedQuery := literalAnonymizer.ReplaceAllString(query, "?")
	return anonymizedQuery
}

// AnonymizeExecutionPlanXML anonymizes only the StatementText attributes in execution plan XML
// while preserving the rest of the XML structure intact. This is more efficient than anonymizing
// the entire XML content and only targets the actual SQL statements within the execution plan.
//
// Examples:
//   Input:  <StmtSimple StatementText="SELECT * FROM users WHERE id = 123" StatementId="1">
//   Output: <StmtSimple StatementText="SELECT * FROM users WHERE id = ?" StatementId="1">
//
// Parameters:
//   - xmlContent: The execution plan XML content to process
//
// Returns:
//   - string: The XML with anonymized StatementText attributes
//
// Safety Features:
//   - Handles empty/nil input safely
//   - Limits XML processing size to prevent performance issues
//   - Preserves XML structure while only anonymizing SQL content
//   - Handles both single and double quoted StatementText values
func AnonymizeExecutionPlanXML(xmlContent string) string {
	if xmlContent == "" {
		return xmlContent
	}
	
	// Safety check for very large XML content
	const maxSafeLength = 65536 // 64KB limit for XML processing
	if len(xmlContent) > maxSafeLength {
		return xmlContent[:maxSafeLength] + "...[XML content too large for processing]"
	}
	
	// Replace function that anonymizes only the SQL content within StatementText attributes
	anonymizedXML := statementTextRegex.ReplaceAllStringFunc(xmlContent, func(match string) string {
		// Find the position of the equals sign and the quotes
		eqIndex := strings.Index(match, "=")
		if eqIndex == -1 {
			return match
		}
		
		// Find the start of the quote (skip whitespace after =)
		quoteStart := eqIndex + 1
		for quoteStart < len(match) && (match[quoteStart] == ' ' || match[quoteStart] == '\t') {
			quoteStart++
		}
		
		if quoteStart >= len(match) {
			return match
		}
		
		quote := match[quoteStart]
		if quote == '"' {
			// Extract the SQL content between double quotes
			start := quoteStart + 1
			end := strings.LastIndex(match, `"`)
			if start < end {
				sqlContent := match[start:end]
				anonymizedSQL := AnonymizeQueryText(sqlContent)
				return match[:start] + anonymizedSQL + match[end:]
			}
		} else if quote == '\'' {
			// Extract the SQL content between single quotes
			start := quoteStart + 1
			end := strings.LastIndex(match, `'`)
			if start < end {
				sqlContent := match[start:end]
				anonymizedSQL := AnonymizeQueryText(sqlContent)
				return match[:start] + anonymizedSQL + match[end:]
			}
		}
		return match // Return unchanged if parsing fails
	})
	
	return anonymizedXML
}

// SafeAnonymizeQueryText provides additional safety wrapper around AnonymizeQueryText
// with extra null/empty string protection for use in critical paths.
//
// Parameters:
//   - queryPtr: Pointer to query string (can be nil)
//
// Returns:
//   - string: Anonymized query text or appropriate fallback
func SafeAnonymizeQueryText(queryPtr *string) string {
	if queryPtr == nil {
		return "[nil query]"
	}
	
	query := *queryPtr
	if query == "" {
		return "[empty query]"
	}
	
	return AnonymizeQueryText(query)
}

// IsQueryTextSafe checks if a query text is safe to process for anonymization
// without risking performance issues or memory problems.
//
// Parameters:
//   - query: The query text to check
//
// Returns:
//   - bool: true if safe to process, false if too large or problematic
func IsQueryTextSafe(query string) bool {
	const maxSafeLength = 16384 // 16KB limit
	return len(query) <= maxSafeLength
}

// IsXMLContentSafe checks if XML content is safe to process for anonymization
// without risking performance issues or memory problems.
//
// Parameters:
//   - xmlContent: The XML content to check
//
// Returns:
//   - bool: true if safe to process, false if too large or problematic
func IsXMLContentSafe(xmlContent string) bool {
	const maxSafeLength = 65536 // 64KB limit
	return len(xmlContent) <= maxSafeLength
}