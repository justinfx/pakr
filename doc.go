/*

pakr (package resolver) provides a way to generically define both an index of total available packages,
and a list of requirements/selections from that index, and resolve a package solution if possible.

- A "product" is a unique name that will have 1 or more package versions.

- A "package" is a specific version of a product, with specific dependencies.
*/
package pakr
