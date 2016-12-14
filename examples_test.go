package pakr

import (
	"fmt"
	"sort"
)

func Example_solve() {
	P := NewPackage

	// Define an index that represents all available Packages
	// and their dependencies
	index := []Dependency{
		{
			P("A", "1.0.0"), []Packages{
				{P("B", "1.0.0")},
			},
		},
		{
			P("A", "2.0.0"), []Packages{
				{P("B", "2.0.0")},
			},
		},
		{
			P("B", "2.0.0"), []Packages{
				{P("C", "1.0.0")},
			},
		},
	}

	// Our specific constraints that we want to solve
	required := Packages{P("A", "2.0.0")}

	// Just set our specific requires to be each Package defined
	// in the Dependency index
	resolver := NewResolver(required, index)

	solved, err := resolver.Resolve()
	if err != nil {
		panic(err.Error())
	}
	if !solved {
		panic("Resolver was expected to succeed, but failed.")
	}

	solutions := resolver.Solution()
	sort.Sort(solutions)

	for _, pkg := range solutions {
		fmt.Println(pkg.PackageName())
	}
	// Output:
	// A-2.0.0
	// B-2.0.0
	// C-1.0.0
}

func Example_conflict() {
	P := NewPackage

	// Define an index that represents all available Packages
	// and their dependencies
	index := []Dependency{
		{
			P("A", "1.0.0"), []Packages{
				{P("C", "1.0.0")},
			},
		},
		{
			P("B", "1.0.0"), []Packages{
				{P("C", "2.0.0")},
			},
		},
	}

	// Our specific constraints that we want to solve
	required := Packages{
		P("A", "1.0.0"),
		P("B", "1.0.0")}

	// Just set our specific requires to be each Package defined
	// in the Dependency index
	resolver := NewResolver(required, index)

	solved, err := resolver.Resolve()
	if err != nil {
		panic(err.Error())
	}
	if solved {
		panic("Resolver was expected to fail, but succeeded.")
	}

	fmt.Println("The following requirements cannot be satisfied:")
	for _, c := range resolver.Conflicts() {
		fmt.Printf("    %s\n", c.PackageName())
	}

	fmt.Println("\nDetails:")
	detailed, _ := resolver.DetailedConflicts()
	fmt.Println(detailed)
	// Output:
	// The following requirements cannot be satisfied:
	//     A-1.0.0
	//     B-1.0.0
	//
	// Details:
	// Package A-1.0.0 depends on one of (C-1.0.0)
	// Package B-1.0.0 depends on one of (C-2.0.0)
	// Package C-2.0.0 conflicts with (C-1.0.0)
}
