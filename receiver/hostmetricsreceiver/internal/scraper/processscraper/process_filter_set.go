package processscraper

type processFilterSet struct {
	filters [] processFilter
}

func (p *processFilterSet) MatchesExecutable(executableName string, executablePath string, indexList []int) []int {
	var matches []int

	for _, i := range indexList {
		if p.filters[i].MatchesExecutable(executableName, executablePath) {
			matches = append(matches, i)
		}
	}

	return matches
}

func (p *processFilterSet) MatchesCommand(command string, commandLine string, indexList []int) []int {
	var matches []int

	for _, i := range indexList {
		if p.filters[i].MatchesCommand(command, commandLine) {
			matches = append(matches, i)
		}
	}

	return matches
}

func (p *processFilterSet) MatchesOwner(owner string, indexList []int) []int {
	var matches []int

	for _, i := range indexList {
		if p.filters[i].MatchOwner(owner) {
			matches = append(matches, i)
		}
	}

	return matches
}

func (p *processFilterSet) MatchesPid(pid int32) []int {
	var matches []int

	for i, f := range p.filters {
		if f.MatchesPid(pid) {
			matches = append(matches, i)
		}
	}

	return matches
}

func createFilters(filterConfigs []FilterConfig) (*processFilterSet, error) {
	var filters []processFilter
	for _, filterConfig := range filterConfigs {
		filter, err := createFilter(filterConfig)
		if err != nil {
			return nil, err
		}
		filters = append(filters, filter)
	}

	// if there are no filters, create an empty filter that matches all processes
	if len(filters) == 0 {
		filters = append(filters, processFilter{})
	}
	return &processFilterSet{filters: filters}, nil
}

