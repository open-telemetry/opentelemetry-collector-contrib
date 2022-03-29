package processscraper

type processFilterSet struct {
	filters [] processFilter
}

// includeExecutable returns an int array with the index of all filters that match the input executable values
// only indexes provided in indexList are checked
func (p *processFilterSet) includeExecutable(executableName string, executablePath string, indexList []int) []int {
	var matches []int

	for _, i := range indexList {
		if p.filters[i].includeExecutable(executableName, executablePath) {
			matches = append(matches, i)
		}
	}

	return matches
}

// includeCommand returns an int array with the index of all filters that match the input command values
// only indexes provided in indexList are checked
func (p *processFilterSet) includeCommand(command string, commandLine string, indexList []int) []int {
	var matches []int

	for _, i := range indexList {
		if p.filters[i].includeCommand(command, commandLine) {
			matches = append(matches, i)
		}
	}

	return matches
}

// MatchesOwner returns an int array with the index of all filters that match the input owner value
// only indexes provided in indexList are checked
func (p *processFilterSet) includeOwner(owner string, indexList []int) []int {
	var matches []int

	for _, i := range indexList {
		if p.filters[i].includeOwner(owner) {
			matches = append(matches, i)
		}
	}

	return matches
}

// MatchesPid returns an int array with the index of all filters that match the input pid value
func (p *processFilterSet) includePid(pid int32) []int {
	var matches []int

	for i, f := range p.filters {
		if f.includePid(pid) {
			matches = append(matches, i)
		}
	}

	return matches
}

// createFilters creates a processFilterSet based on an input config.
func createFilters(filterConfigs []FilterConfig) (*processFilterSet, error) {
	var filters []processFilter
	for _, filterConfig := range filterConfigs {
		filter, err := createFilter(filterConfig)
		if err != nil {
			return nil, err
		}
		filters = append(filters, *filter)
	}

	// if there are no filters, create an empty filter that matches all processes
	if len(filters) == 0 {
		filters = append(filters, processFilter{})
	}
	return &processFilterSet{filters: filters}, nil
}

