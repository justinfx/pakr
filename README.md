## pakr

A package version resolver library, built on top of bindings for picosat

[![GoDoc](https://godoc.org/github.com/justinfx/pakr?status.svg)](https://godoc.org/github.com/justinfx/pakr)

pakr (package resolver) provides a way to generically define both an index of total available packages,
and a list of requirements/selections from that index, and resolve a package solution if possible.

- A "product" is a unique name that will have 1 or more package versions.

- A "package" is a specific version of a product, with specific dependencies.

### Examples

```go
P := NewPackage
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
```

Output:

```
The following requirements cannot be satisfied:
    A-1.0.0
    B-1.0.0

Details:
Package A-1.0.0 depends on one of (C-1.0.0)
Package B-1.0.0 depends on one of (C-2.0.0)
Package C-2.0.0 conflicts with (C-1.0.0)
```
