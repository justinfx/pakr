package pakr

import (
	"fmt"
	"strings"

	"github.com/justinfx/pigosat"
)

// Packager interface represents an object that defines
// its Product name, Package name, and version identifier.
// Packages are variants of the same Product, with a different version number.
// A Product has 1 or more Packages, defining different versions.
type Packager interface {
	ProductName() string
	PackageName() string
	Version() string
}

// A slice of Packagers that supports sorting
type Packages []Packager

// Len returns the number of Package objects contained
func (p Packages) Len() int {
	return len(p)
}

// Less compares to indexes of a Packages instance
func (p Packages) Less(i, j int) bool {
	return p[i].PackageName() < p[j].PackageName()
}

// Swaps two indices of a Packages instance
func (p Packages) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

func (p Packages) String() string {
	strs := make([]string, len(p))
	for i, p := range p {
		strs[i] = p.PackageName()
	}
	return strings.Join(strs, ", ")
}

// Defines a Package, and all of its direct dependencies.
// Dependencies are lists of expanded Package version ranges. So
// for each package that is a dependency, all allowable Package
// versions are listed.
type Dependency struct {
	Target   Packager
	Requires []Packages
}

// Return a new Dependencies instance, with a Packager
// and an empty list of Version sets
func NewDependency(p Packager) *Dependency {
	return &Dependency{p, make([]Packages, 0)}
}

// AddVersionSet appends a list of Package version of a Product,
// to the Depends list.
func (p *Dependency) AddVersionSet(vers Packages) {
	if p.Requires == nil {
		p.Requires = []Packages{vers}
	} else {
		p.Requires = append(p.Requires, vers)
	}
}

// A Package is a specific version of a Product
type Package struct {
	product     string
	version     string
	packageName string
}

// NewPackage creates a new Package with a product name and version
func NewPackage(productName, version string) *Package {
	return &Package{product: productName, version: version}
}

// ProductName returns the unversioned name of the product
func (p *Package) ProductName() string {
	return p.product
}

// PackageName returns the <Name>-<Version> of the Package
func (p *Package) PackageName() string {
	if p.packageName == "" {
		p.packageName = fmt.Sprintf("%s-%s", p.product, p.version)
	}
	return p.packageName
}

// Version returns the version identifier
func (p *Package) Version() string {
	return p.version
}

// String returns the string name of the Package
func (p *Package) String() string {
	return p.PackageName()
}

// packagesToIds takes a slice of Package instances, and a
// stringIdMap, and returns the mapped ids for the Package names
func packagesToIds(a []Packager, idMap *stringIdMap) []pigosat.Literal {
	ids := make([]pigosat.Literal, len(a))
	for i, s := range a {
		ids[i] = idMap.StringToId(s.PackageName())
	}
	return ids
}

// A set of Packagers
type packageSet map[string]Packager

// A struct that tracks mappings of Product names to package sets,
// and Package names to Packagers.
// Create with NewProductMap()
type ProductMap struct {
	prods map[string]packageSet
	pkgs  map[string]Packager
}

// ProductMap tracks Packages, organizing them as a mapping of
// Product names to sets of Packages (version of each Product)
func NewProductMap() *ProductMap {
	return &ProductMap{
		prods: map[string]packageSet{},
		pkgs:  map[string]Packager{},
	}
}

// Return the number of tracked products
func (m *ProductMap) NumProducts() int {
	return len(m.prods)
}

// Return the number of tracked packages
func (m *ProductMap) NumPackages() int {
	return len(m.pkgs)
}

// Add a Package to the mapping
// It will be organized by its Product name
func (m *ProductMap) Add(p Packager) {
	prodName := p.ProductName()
	pkgName := p.PackageName()

	set, ok := m.prods[prodName]
	if !ok {
		m.prods[prodName] = packageSet{pkgName: p}
	} else {
		set[pkgName] = p
	}
	// Index the Package by its name
	m.pkgs[pkgName] = p
}

// Retrieve all Packages mapped by their Product name
func (m *ProductMap) Packages(productName string) []Packager {
	set, ok := m.prods[productName]
	if !ok {
		return nil
	}
	packs := make([]Packager, len(set))
	i := 0
	for _, p := range set {
		packs[i] = p
		i++
	}
	return packs
}

// Looks up and returns a Package by its PackageName
// Returns a non-nil error if not found
func (m *ProductMap) PackageByName(packageName string) (Packager, error) {
	p, ok := m.pkgs[packageName]
	if ok {
		return p, nil
	}
	return p, fmt.Errorf("Package %q does not exist", packageName)
}

// A constant defining a relationship of a Packages contribution
// to a set of requirements
type Relation string

const (
	Required  Relation = `Required`
	Conflicts Relation = `Conflicts`
	Depends   Relation = `Depends`
	Restricts Relation = `Restricted`
)

// PackageRelation relationship of either one Package
// to the overall requirements, or two packages to eachother
type PackageRelation struct {
	Packages Packages
	Relates  Relation
}

// Generate the string representation of the relationship
// as a descriptive phrase.
func (r *PackageRelation) String() string {
	switch r.Relates {
	case Required:
		return fmt.Sprintf("Package %s is required", r.Packages[0].PackageName())
	case Restricts:
		return fmt.Sprintf("Package %s is not allowed", r.Packages[0].PackageName())
	case Depends:
		return fmt.Sprintf("Package %s depends on one of (%s)", r.Packages[0].PackageName(), r.Packages[1:])
	case Conflicts:
		return fmt.Sprintf("Package %s conflicts with (%s)", r.Packages[0].PackageName(), r.Packages[1:])
	}
	return ""
}

// PackageRelations is a list of PackageRelation objects
type PackageRelations []*PackageRelation

// Generate all descriptive phrases for contained relationships,
// separated by newlines
func (p PackageRelations) String() string {
	if p == nil {
		return ""
	}
	strs := make([]string, len(p))
	for i, rel := range p {
		strs[i] = rel.String()
	}
	return strings.Join(strs, "\n")
}

// Take a list of Dependency objects, and produce a flat
// list of Packagers from all targets and dependencies.
// Basically a list of every reference to every Packager.
func flattenDependencies(deps []Dependency) Packages {
	packMap := make(map[string]Packager, len(deps))
	for _, d := range deps {
		packMap[d.Target.PackageName()] = d.Target
		if d.Requires == nil {
			continue
		}

		for _, verList := range d.Requires {
			for _, ver := range verList {
				packMap[ver.PackageName()] = ver
			}
		}
	}

	packList := make(Packages, len(packMap))
	i := 0
	for _, p := range packMap {
		packList[i] = p
		i++
	}
	return packList
}
