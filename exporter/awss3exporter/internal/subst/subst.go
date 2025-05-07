package subst

import (
	"maps"
	"regexp"
	"slices"
)

var re = regexp.MustCompile(`\$\{([^}]+)\}`)

func Subst(txt string, f func(key string) string) string {
	return re.ReplaceAllStringFunc(txt, func(m string) string {
		key := m[2 : len(m)-1]
		return f(key)
	})
}

func Vars(txt string) []string {
	vars := map[string]bool{}
	m := re.FindAllStringSubmatch(txt, -1)
	for _, s := range m {
		vars[s[1]] = true
	}
	return slices.Sorted(maps.Keys(vars))
}
