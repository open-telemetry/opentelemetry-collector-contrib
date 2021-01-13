package metricsdimensioncache

// DimensionCache is trie of all known combinations dimensions.
// It is used to
type DimensionCache struct {
	root *Dimension
}

// Dimension
type Dimension struct {
	children map[string]*Dimension
	json     *string
}

func NewDimensionCache() *DimensionCache {
	return &DimensionCache{
		root: &Dimension{children: make(map[string]*Dimension)},
	}
}

func (c *DimensionCache) Empty() bool {
	return len(c.root.children) == 0
}

func (c *DimensionCache) InsertDimensions(ks ...string) *Dimension {
	return c.root.InsertDimensions(ks...)
}

func (d *Dimension) InsertDimensions(ks ...string) *Dimension {
	childDimension := d
	for _, k := range ks {
		childDimension = childDimension.insertNextDimension(k)
	}
	return childDimension
}

func (parent *Dimension) insertNextDimension(k string) *Dimension {
	child, ok := parent.children[k]
	if !ok {
		child = &Dimension{children: make(map[string]*Dimension)}
		parent.children[k] = child
	}
	return child
}

func (parent *Dimension) FoundCachedDimensionKey() bool {
	return parent.json != nil
}

func (parent *Dimension) GetCachedDimensionKey() string {
	return *parent.json
}

func (parent *Dimension) SetCachedDimensionKey(k string) {
	parent.json = &k
}
