package pakr

import (
	"sort"
	"testing"
)

func TestPackageSuccess(t *testing.T) {
	P := NewPackage

	// Our specific package requirements and their deps
	index := []Dependency{
		{
			P("A", "1.0.0"), []Packages{
				{P("B", "1.2.3"), P("B", "1.2.5"), P("B", "1.2.9")},
				{P("C", "2.0.0"), P("C", "2.1.0"), P("C", "2.2.0")},
			},
		},
		{
			P("C", "2.1.0"), []Packages{
				{P("D", "5.0.0"), P("D", "5.0.1")},
				{P("E", "2.0.0"), P("E", "3.0.0")},
			},
		},
		{
			P("D", "5.0.1"), []Packages{
				{P("B", "1.2.3"), P("B", "1.2.9")},
				{P("E", "3.0.0")},
			},
		},
		{
			P("F", "0.5.5"), []Packages{
				{P("C", "2.1.0")},
				{P("X", "1.5.0")},
				{P("Y", "2.0.0")},
			},
		},
		{P("Z", "1.0.0"), nil},
	}

	// Just set our specific requires to be each Package defined
	// in the Dependency index
	requires := make(Packages, len(index))

	for i, dep := range index {
		requires[i] = dep.Target
		i++
	}

	resolver := NewResolver(requires, index)

	solved, err := resolver.Resolve()
	if err != nil {
		t.Fatal(err.Error())
	}
	if !solved || !resolver.Solved() {
		t.Fatal("Resolver was expected to succeed, but failed.")
	}

	solution := resolver.Solution()
	sort.Sort(solution)
	for _, pkg := range solution {
		t.Log(pkg.PackageName())
	}
}

func TestPackageConflict(t *testing.T) {
	P := NewPackage

	// Our specific package requirements and their deps
	index := []Dependency{
		{
			P("A", "1.0.0"), []Packages{
				{P("B", "1.2.3"), P("B", "1.2.5"), P("B", "1.2.9")},
				{P("C", "2.0.0"), P("C", "2.1.0"), P("C", "2.2.0")},
			},
		},
		{
			P("C", "2.1.0"), []Packages{
				{P("D", "5.0.0"), P("D", "5.0.1")},
				{P("E", "2.0.0"), P("E", "3.0.0")},
			},
		},
		{
			P("D", "5.0.1"), []Packages{
				{P("B", "1.2.3"), P("B", "1.2.9")},
				{P("E", "3.0.0")},
			},
		},
		{
			P("F", "0.5.5"), []Packages{
				{P("B", "1.2.5")}, // Conflict with D-5.0.1
				{P("C", "2.1.0")},
				{P("X", "1.5.0")},
				{P("Y", "2.0.0")},
			},
		},
		{P("Z", "1.0.0"), nil},
	}

	// Just set our specific requires to be each Package defined
	// in the Dependency index
	requires := make(Packages, len(index))

	for i, dep := range index {
		requires[i] = dep.Target
		i++
	}

	resolver := NewResolver(requires, index)

	solved, err := resolver.Resolve()
	if err != nil {
		t.Fatal(err.Error())
	}
	if solved || resolver.Solved() {
		t.Fatal("Resolver was expected to fail, but succeeded.")
	}

	detailed, err := resolver.DetailedConflicts()
	if err != nil {
		t.Errorf("Error from DetailedConflicts: %s", err.Error())
	}
	t.Log(detailed)

	// Check failed assumptions
	if resolver.IsPackageNameConflict("C-2.1.0") {
		t.Error("Expected C-2.1.0 to *not* be one of the failed assumptions")
	}

	expected := []string{"D-5.0.1", "F-0.5.5"}
	for _, name := range expected {
		if !resolver.IsPackageNameConflict(name) {
			t.Errorf("Expected %s to be one of the failed assumptions", name)
		}
	}

	conflicts := resolver.Conflicts()
	sort.Sort(conflicts)
	if len(conflicts) != 2 {
		t.Errorf("Expected 2 failed packages, but got %d", len(conflicts))
	}
	for i, actual := range conflicts {
		t.Logf("Conflicts: %s", actual)
		if expected[i] != actual.PackageName() {
			t.Errorf("Expected %q package failure, but got %q", expected[i], actual.PackageName())
		}
	}

}

func TestChooseHighVersions(t *testing.T) {
	P := NewPackage

	// Our specific package requirements and their deps
	index := []Dependency{
		{P("A", "2.0.0"), []Packages{{P("B", "2.0.0"), P("B", "1.0.0")}}},
		{P("A", "1.0.0"), []Packages{{P("B", "2.0.0"), P("B", "1.0.0")}}},
		{P("B", "2.0.0"), []Packages{{P("C", "2.0.0"), P("C", "1.0.0")}}},
		{P("B", "1.0.0"), []Packages{{P("C", "2.0.0"), P("C", "1.0.0")}}},
		{P("C", "2.0.0"), []Packages{}},
		{P("C", "1.0.0"), []Packages{}},
	}

	// Just set our specific requires to be each Package defined
	// in the Dependency index
	resolver := NewSortResolver(Packages{P("A", "2.0.0")}, index, ResolveSortHigh)

	solved, err := resolver.Resolve()
	if err != nil {
		t.Fatal(err.Error())
	}
	if !solved || !resolver.Solved() {
		t.Fatal("Resolver was expected to succeed, but failed.")
	}

	solution := resolver.Solution()
	sort.Sort(solution)
	for _, pkg := range solution {
		t.Log(pkg.PackageName())
	}
	expected := Packages{P("A", "2.0.0"), P("B", "2.0.0"), P("C", "2.0.0")}
	if len(expected) != len(solution) {
		t.Fatalf("Expected length of %d but got %d", len(expected), len(solution))
	}
	for i, p := range expected {
		if solution[i].PackageName() != p.PackageName() {
			t.Fatalf("Expected solution package %s, but got %s", p.PackageName(), solution[i].PackageName())
		}
	}
}

func TestChooseLowVersions(t *testing.T) {
	P := NewPackage

	// Our specific package requirements and their deps
	index := []Dependency{
		{P("A", "1.0.0"), []Packages{{P("B", "1.0.0"), P("B", "2.0.0")}}},
		{P("A", "2.0.0"), []Packages{{P("B", "1.0.0"), P("B", "2.0.0")}}},
		{P("B", "1.0.0"), []Packages{{P("C", "1.0.0"), P("C", "2.0.0")}}},
		{P("B", "2.0.0"), []Packages{{P("C", "1.0.0"), P("C", "2.0.0")}}},
		{P("C", "1.0.0"), []Packages{}},
		{P("C", "2.0.0"), []Packages{}},
	}

	// Just set our specific requires to be each Package defined
	// in the Dependency index
	resolver := NewSortResolver(Packages{P("A", "1.0.0")}, index, ResolveSortLow)

	solved, err := resolver.Resolve()
	if err != nil {
		t.Fatal(err.Error())
	}
	if !solved || !resolver.Solved() {
		t.Fatal("Resolver was expected to succeed, but failed.")
	}

	solution := resolver.Solution()
	sort.Sort(solution)
	for _, pkg := range solution {
		t.Log(pkg.PackageName())
	}
	expected := Packages{P("A", "1.0.0"), P("B", "1.0.0"), P("C", "1.0.0")}
	if len(expected) != len(solution) {
		t.Fatalf("Expected length of %d but got %d", len(expected), len(solution))
	}
	for i, p := range expected {
		if solution[i].PackageName() != p.PackageName() {
			t.Fatalf("Expected solution package %s, but got %s", p.PackageName(), solution[i].PackageName())
		}
	}
}
