package exposition

import (
	"errors"
	"regexp"
	"sort"
	"strings"

	"github.com/Denki77/metricshell/implementation/internal/snapshot"
)

type SelectorKind string

const (
	SelectorName   SelectorKind = "name"
	SelectorPrefix SelectorKind = "prefix"
)

var ErrSelector = errors.New("invalid metric-family selector")
var selectorMetricName = regexp.MustCompile(`^[a-zA-Z_:][a-zA-Z0-9_:]*$`)

type Selector struct {
	Kind  SelectorKind
	Value string
}

func ParseSelector(value string) (Selector, error) {
	kind, metric, found := strings.Cut(value, ":")
	selector := Selector{Kind: SelectorKind(kind), Value: metric}
	if !found || (selector.Kind != SelectorName && selector.Kind != SelectorPrefix) || !selectorMetricName.MatchString(metric) {
		return Selector{}, ErrSelector
	}
	return selector, nil
}

type Filter struct {
	include []Selector
	exclude []Selector
}

func NewFilter(includes, excludes []string) (*Filter, error) {
	include, err := normalizeSelectors(includes)
	if err != nil {
		return nil, err
	}
	exclude, err := normalizeSelectors(excludes)
	if err != nil {
		return nil, err
	}
	return &Filter{include: include, exclude: exclude}, nil
}

func (filter *Filter) Includes(family snapshot.Family) bool {
	name := family.Name()
	if len(filter.include) > 0 && !matchesAny(filter.include, name) {
		return false
	}
	return !matchesAny(filter.exclude, name)
}

func (filter *Filter) Counts(families []snapshot.Family) (included, excluded int) {
	for _, family := range families {
		if filter.Includes(family) {
			included++
		} else {
			excluded++
		}
	}
	return included, excluded
}

func (filter *Filter) RuleCounts() (include, exclude int) {
	return len(filter.include), len(filter.exclude)
}

func normalizeSelectors(values []string) ([]Selector, error) {
	unique := make(map[Selector]struct{}, len(values))
	for _, value := range values {
		selector, err := ParseSelector(value)
		if err != nil {
			return nil, err
		}
		unique[selector] = struct{}{}
	}
	result := make([]Selector, 0, len(unique))
	for selector := range unique {
		result = append(result, selector)
	}
	sort.Slice(result, func(left, right int) bool {
		if result[left].Kind == result[right].Kind {
			return result[left].Value < result[right].Value
		}
		return result[left].Kind < result[right].Kind
	})
	return result, nil
}

func matchesAny(selectors []Selector, name string) bool {
	for _, selector := range selectors {
		if selector.Kind == SelectorName && name == selector.Value || selector.Kind == SelectorPrefix && strings.HasPrefix(name, selector.Value) {
			return true
		}
	}
	return false
}
