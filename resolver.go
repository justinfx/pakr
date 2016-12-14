package pakr

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"math/big"
	"sort"
	"strconv"
	"strings"

	"github.com/justinfx/pigosat"
)

// Specifies a sort operation to be performed by
// the Resolver on the loaded package index, before
// solving.
type resolveSort int

const (
	// Leave the package index in its original order
	ResolveSortNone resolveSort = iota
	// Sort packages to prefer lower versions first
	ResolveSortLow
	// Sort packages to prefer higher versions first
	ResolveSortHigh
)

// A Resolver attempts to solve a package solution from
// a given set of constraints and assumptions for a package
// index list
type Resolver struct {
	solver    *pigosat.Pigosat
	idMap     *stringIdMap
	prodMap   *ProductMap
	sortMode  resolveSort
	index     []Dependency
	requires  Packages
	solution  Packages
	conflicts []*PackageRelation
}

// NewResolver creates a new Resolver, from a given package dependency list
func NewResolver(requires Packages, index []Dependency) *Resolver {
	r := &Resolver{requires: requires, index: index}
	if err := r.Initialize(); err != nil {
		// Getting an error here means something is seriously wrong
		// with the pigosat library support
		panic(err)
	}
	return r
}

// NewSortResolver creates a new Resolver, from a given package dependency list.
// Specify a sort order operation to apply to the packages when
// intializing the index. Sort order affects the preference in
// choosing higher vs lower version packages in the solution.
func NewSortResolver(requires Packages, index []Dependency, sortMode resolveSort) *Resolver {
	r := &Resolver{requires: requires, index: index, sortMode: sortMode}
	if err := r.Initialize(); err != nil {
		// Getting an error here means something is seriously wrong
		// with the pigosat library support
		panic(err)
	}
	return r
}

// Set the package dependency list.
// Resets the internal solver and state.
func (r *Resolver) SetRequirements(requires Packages) {
	r.requires = requires
	if err := r.Initialize(); err != nil {
		// Getting an error here means something is seriously wrong
		// with the pigosat library support
		panic(err)
	}
}

// Set the package dependency list.
// Resets the internal solver and state.
func (r *Resolver) SetPackageIndex(index []Dependency) {
	r.index = index
	if err := r.Initialize(); err != nil {
		// Getting an error here means something is seriously wrong
		// with the pigosat library support
		panic(err)
	}
}

// Resets all internal state, and initializes based
// on the currently set package dependency requirements
// This gets called automatically when calling SetRequirements()
func (r *Resolver) Initialize() error {
	opts := &pigosat.Options{EnableTrace: true}

	var err error
	r.solver, err = pigosat.New(opts)
	if err != nil {
		return fmt.Errorf("Failed to initialize a resolver object from pigosat: %s", err.Error())
	}

	r.idMap = newStringIdMap()
	r.prodMap = NewProductMap()

	if r.index == nil {
		return nil
	}

	idMap := r.idMap
	prodMap := r.prodMap

	// Preload the stringIdMap
	if r.sortMode != ResolveSortNone {
		flat := flattenDependencies(r.index)
		if r.sortMode == ResolveSortLow {
			sort.Sort(sort.Reverse(flat))
		} else {
			sort.Sort(flat)
		}
		for _, pack := range flat {
			idMap.StringToId(pack.PackageName())
		}
	}

	// All of the SAT clauses we will build up
	clauses := pigosat.Formula{}

	tid := pigosat.Literal(0)
	cid := pigosat.Literal(0)

	// Add unit clauses and variable constraints
	for _, dep := range r.index {
		tid = idMap.StringToId(dep.Target.PackageName())
		prodMap.Add(dep.Target)

		if dep.Requires == nil {
			continue
		}

		for _, constraints := range dep.Requires {
			// Add variable constraints
			clause := make([]pigosat.Literal, len(constraints)+1)
			clause[0] = -tid

			for i, ver := range constraints {
				cid = idMap.StringToId(ver.PackageName())
				clause[i+1] = cid
				prodMap.Add(ver)
			}

			clauses = append(clauses, clause)
		}
	}

	// Now add multi-version conflicts
	for name, _ := range prodMap.prods {
		vers := prodMap.Packages(name)
		if vers == nil {
			return fmt.Errorf("Resolve init failure: Version list for product %q was nil", name)
		}

		ids := packagesToIds(vers, idMap)
		for _, conflict := range buildConflictClauses(ids) {
			clauses = append(clauses, conflict)
		}
	}

	// Hint the solver at the size of variables, since we
	// just built up a Package index.
	r.solver.Adjust(idMap.Len())

	// Now actually add all the clauses that we had built up,
	// into the solver.
	// fmt.Printf("Clauses: %v\n", clauses)
	r.solver.AddClauses(clauses)

	// fmt.Printf("# variables == %d\n", r.solver.Variables())
	// fmt.Printf("# clauses == %d\n", r.solver.AddedOriginalClauses())

	return nil
}

// addRequires applies the Packages stored as requirements,
// as assumptions to the solver. These assumptions are valid
// only for one call to Resolve at a time.
func (r *Resolver) addRequires() {
	if r.requires == nil {
		return
	}
	var tid pigosat.Literal
	for _, p := range r.requires {
		tid = r.idMap.StringToId(p.PackageName())
		r.solver.Assume(tid)
	}
}

// Returns the last successfully resolved solution of packages
func (r *Resolver) Solution() Packages {
	return r.solution
}

// Attempt to resolve a package solution with the currently set criteria.
// Returns a bool indicating whether the Resolver succeeded or conflicted.
// Returns a non-nil error if there was an internal error.
func (r *Resolver) Resolve() (bool, error) {
	r.solution = Packages{}
	r.conflicts = nil

	if r.solver == nil {
		return false, errors.New("Requirements not set. Solver not initialized.")
	}

	// Push the fixed requirements into the solver
	r.addRequires()

	status, solution := r.solver.Solve()
	if status != pigosat.Satisfiable {
		return false, nil
	}

	var (
		pkgName string
		pkg     Packager
		err     error
	)
	// Remap the literal ids from the solution back into
	// the original objects
	for i := 1; i < len(solution); i++ {
		if solution[i] {
			pkgName = r.idMap.IdToString(pigosat.Literal(i))
			if pkg, err = r.prodMap.PackageByName(pkgName); err != nil {
				return false, fmt.Errorf("Resolve failed to look up package by name %q: %s",
					pkgName, err.Error())
			}
			r.solution = append(r.solution, pkg)
		}
	}
	return true, nil
}

// Returns the returned by the last call to Resolve(),
// indicating whether the current requirements are solved or not.
func (r *Resolver) Solved() bool {
	if r.solver == nil {
		return false
	}

	return r.solver.Res() == pigosat.Satisfiable
}

// Add a package as a requirement that must be satisfied by the solver.
// This addition is only valid until the next call to Resolve(),
// after which it will be removed.
func (r *Resolver) RequireTemp(p Packager) {
	tid := r.idMap.StringToId(p.PackageName())
	r.solver.Assume(tid)
}

// Return true if a given required package (by name) caused the Resolver
// to fail. Only makes sense to call this after having called Resolve()
// and finding that the resolve was not successful.
func (r *Resolver) IsPackageNameConflict(packageName string) bool {
	id, err := r.idMap.GetId(packageName)
	if err != nil {
		return false
	}
	return r.solver.FailedAssumption(id)
}

// Return true if a given required Package caused the Resolver
// to fail. Only makes sense to call this after having called Resolve()
// and finding that the resolve was not successful.
func (r *Resolver) IsPackageConflict(p Packager) bool {
	id, err := r.idMap.GetId(p.PackageName())
	if err != nil {
		return false
	}
	return r.solver.FailedAssumption(id)
}

// Returns a package list of all Packages that were requirements, or added with
// RequireTemp(), that caused the Resolver to fail. Only makes sense to call this
// after having called Resolve() and finding that the resolve was not successful.
func (r *Resolver) Conflicts() Packages {
	ids := r.solver.FailedAssumptions()
	packs := make(Packages, len(ids))
	var (
		name string
		pak  Packager
	)
	for i, id := range ids {
		name = r.idMap.IdToString(id)
		pak, _ = r.prodMap.PackageByName(name)
		packs[i] = pak
	}
	return packs
}

// If the previous call to Resolve() returned false, meaning the
// currently requirements are no solvable, then this method builds
// a list of the packages involved in the conflict.
//
// Returns a slice of PackageRelations, which describe 1 or 2 packages,
// and a descriptive Relation flag
func (r *Resolver) DetailedConflicts() (PackageRelations, error) {
	if r.Solved() {
		return PackageRelations{}, nil
	}

	if r.conflicts != nil {
		return r.conflicts, nil
	}

	var buf bytes.Buffer
	if err := r.solver.WriteClausalCore(&buf); err != nil {
		return nil, fmt.Errorf("Failed to generate detailed conflict report: %s", err.Error())
	}

	pkgs, err := r.cnfToPackageRelations(&buf)
	if err != nil {
		return nil, err
	}

	r.conflicts = pkgs
	return pkgs, nil
}

// Return a Package by its name.
// If the Package is not known to the Resolver, return an error
func (r *Resolver) PackageByName(packageName string) (Packager, error) {
	return r.prodMap.PackageByName(packageName)
}

// // Sets a given Package known to the Resolver to be more important,
// // when it makes a decision between packages of the same importance.
// func (r *Resolver) SetMoreImportant(p Packager) error {
// 	id, err := r.idMap.GetId(p.PackageName())
// 	if err != nil {
// 		return fmt.Errorf("Package %q does not exist in the Resolver", p.PackageName())
// 	}
// 	r.solver.SetMoreImportant(id)
// 	return nil
// }

// // Sets a given Package (by PackageName) known to the Resolver to be more important,
// // when it makes a decision between packages of the same importance.
// func (r *Resolver) SetMoreImportantName(packageName string) error {
// 	id, err := r.idMap.GetId(packageName)
// 	if err != nil {
// 		return fmt.Errorf("Package %q does not exist in the Resolver", packageName)
// 	}
// 	r.solver.SetMoreImportant(id)
// 	return nil
// }

// // Sets a given Package known to the Resolver to be less important,
// // when it makes a decision between packages of the same importance.
// func (r *Resolver) SetLessImportant(p Packager) error {
// 	id, err := r.idMap.GetId(p.PackageName())
// 	if err != nil {
// 		return fmt.Errorf("Package %q does not exist in the Resolver", p.PackageName())
// 	}
// 	r.solver.SetLessImportant(id)
// 	return nil
// }

// // Sets a given Package (by PackageName) known to the Resolver to be less important,
// // when it makes a decision between packages of the same importance.
// func (r *Resolver) SetLessImportantName(packageName string) error {
// 	id, err := r.idMap.GetId(packageName)
// 	if err != nil {
// 		return fmt.Errorf("Package %q does not exist in the Resolver", packageName)
// 	}
// 	r.solver.SetLessImportant(id)
// 	return nil
// }

// Reads CNF format, written by solvers clausal core output, and
// parses back into Packages. These parsed packages are organized
// into relationship structures that define the progression that
// led to a failed solve.
func (r *Resolver) cnfToPackageRelations(stream io.Reader) (PackageRelations, error) {
	var (
		line   string
		clause int
		err    error
		parsed int64
		rels   PackageRelations
	)

	buf := bufio.NewScanner(stream)
	for buf.Scan() {
		line = buf.Text()
		if len(line) == 0 {
			continue
		}

		switch string(line[0]) {

		case "c":
			// "c" is a commented line
			continue

		case "p":
			// "p" indicates the preamble, defining structure sizes
			fields := strings.Fields(line)
			size, err := strconv.ParseInt(fields[len(fields)-1], 10, 32)
			if err != nil {
				return nil, err
			}
			rels = make(PackageRelations, size, size)

		default:
			// parse a clause, one per line
			fields := strings.Fields(line)
			if len(fields) == 0 {
				return nil, fmt.Errorf("Expected to parse literals, but got none in line: %s", line)
			}

			lits := make([]int, len(fields)-1)
			negs := 0
			for i, f := range fields {
				if f == "0" {
					continue
				}
				if parsed, err = strconv.ParseInt(f, 10, 32); err != nil {
					return nil, fmt.Errorf("Error parsing int %r from line %r", f, line)
				}
				if parsed < 0 {
					negs++
				}
				lits[i] = int(parsed)
			}

			sort.Ints(lits)

			paks := make(Packages, len(lits))
			for i, l := range lits {
				if paks[i], err = r.PackageByName(r.idMap.IdToString(pigosat.Literal(l))); err != nil {
					return nil, fmt.Errorf("Unexpected literal %r in line %r "+
						"could not be mapped back to Package name", l, line)
				}
			}

			var relates Relation
			if len(lits) == 1 {
				// We parsed a single literal
				if lits[0] > 0 {
					relates = Required
				} else {
					relates = Restricts
				}
			} else {
				// We parsed multiple valid literals
				if negs == 1 {
					relates = Depends
				} else if negs > 1 {
					relates = Conflicts
				} else {
					return nil, fmt.Errorf("Unhandled clause type for line: %s", line)
				}
			}
			rels[clause] = &PackageRelation{paks, relates}
			clause++
		}

	}

	if err := buf.Err(); err != nil {
		return nil, err
	}

	return rels, nil
}

// Given a slice of literals, build a list of 2-item clauses
// representing a conflict of each item with any other item in
// the same list.
// Example:
//   Input:  [1, 2, 3]
//   Output: [[-1, -2], [-1, -3], [-2, -3]]
func buildConflictClauses(lits []pigosat.Literal) [][]pigosat.Literal {
	count := len(lits)
	if count <= 1 {
		return [][]pigosat.Literal{}
	}

	num := numCombos(int64(count))
	clauses := make([][]pigosat.Literal, num, num)

	i := 0
	for x := 0; x < (count - 1); x++ {
		for y := x + 1; y < count; y++ {
			clauses[i] = []pigosat.Literal{-lits[x], -lits[y]}
			i++
		}
	}

	return clauses
}

// stringIdMap assigns and tracks unique pigosat.Literal ids that map
// to unique strings
type stringIdMap struct {
	i_map map[pigosat.Literal]string
	s_map map[string]pigosat.Literal
	i     pigosat.Literal
}

// Create a new StringIdMap that can track unique pigosat.Literal ids that map
// to unique string names
func newStringIdMap() *stringIdMap {
	return &stringIdMap{
		map[pigosat.Literal]string{},
		map[string]pigosat.Literal{},
		0,
	}
}

// Len returns the current length of the mapping (number of items mapped)
func (m *stringIdMap) Len() int {
	return len(m.s_map)
}

// StringToId returns a unique id for a given string.
// If the string is already mapped, its existing id will be returned.
// Otherwise a new one will be created, and returned.
// Passing an empty string is considered invalid and always returns 0
func (m *stringIdMap) StringToId(s string) pigosat.Literal {
	if s == "" {
		return 0
	}

	if id, exists := m.s_map[s]; exists {
		return id
	}
	m.i++
	m.s_map[s] = m.i
	m.i_map[m.i] = s
	return m.i
}

// GetId looks up an id for an existing string mapping.
// If no id exists, then return an error
func (m *stringIdMap) GetId(s string) (pigosat.Literal, error) {
	if id, exists := m.s_map[s]; exists {
		return id, nil
	}
	return 0, fmt.Errorf("No id exists for string %q", s)
}

// Return the string for which an existing id maps.
// If no id exists, return an empty string.
func (m *stringIdMap) IdToString(i pigosat.Literal) string {
	if i < 0 {
		i *= -1
	}
	if s, exists := m.i_map[i]; exists {
		return s
	}
	return ""
}

// GetString looks up a string for an existing id mapping.
// If no mapping exists, then return an error
func (m *stringIdMap) GetString(id pigosat.Literal) (string, error) {
	if s, exists := m.i_map[id]; exists {
		return s, nil
	}
	return "", fmt.Errorf("No string exists for id %d", id)
}

// NumCombos returns the number of combination pairs
// given n elements
func numCombos(n int64) int64 {
	t := big.NewInt(0).MulRange(1, n)
	b1 := big.NewInt(0).MulRange(1, n-2)
	b2 := big.NewInt(2)
	return t.Div(t, b1.Mul(b1, b2)).Int64()
}
