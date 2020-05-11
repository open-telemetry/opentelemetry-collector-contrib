package dimensions

import "fmt"

type DimensionUpdate struct {
	Name         string
	Value        string
	Properties   map[string]*string
	TagsToAdd    []string
	TagsToRemove []string
}

func (d *DimensionUpdate) String() string {
	return fmt.Sprintf("{name: %q; value: %q; props: %v; tagsToAdd: %v; tagsToRemove: %v}", d.Name, d.Value, d.Properties, d.TagsToAdd, d.TagsToRemove)
}

func (d *DimensionUpdate) Key() DimensionKey {
	return DimensionKey{
		Name:  d.Name,
		Value: d.Value,
	}
}

// Name is what uniquely identifies a dimension, its name and value
// together.
type DimensionKey struct {
	Name  string
	Value string
}

func (dk DimensionKey) String() string {
	return fmt.Sprintf("[%s/%s]", dk.Name, dk.Value)
}
