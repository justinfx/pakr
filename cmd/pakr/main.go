package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	"sync"

	"github.com/justinfx/pakr"
)

var (
	optIndexPath = flag.String("index", "", "Path to Index/Repo JSON file")
	optReqsPath  = flag.String("reqs", "", "Path to Requirements JSON file")
)

var usage = `Usage:  %s -index <index.json> -reqs <reqs.json>

A helper utility for doing a command-line package resolves using the pakr library.

"index" represents all of the available packages (their versions and requirements)
"req" represents the particular package constraints you want to resolve

Returns a json output indicating whether the solve succeded,
and either the package solution or an error messages explaining the failure.

`

func main() {
	runtime.GOMAXPROCS(2)

	// Handle command line flags
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, usage, os.Args[0])
		flag.PrintDefaults()
	}

	flag.Parse()

	if *optIndexPath == "" {
		log.Fatalln("-index flag is required")
	}

	if *optReqsPath == "" {
		log.Fatalln("-reqs flag is required")
	}

	idxFile, err := os.Open(*optIndexPath)
	if err != nil {
		log.Fatalf("Failed to open Index JSON file: %s", err)
	}
	defer idxFile.Close()

	reqsFile, err := os.Open(*optReqsPath)
	if err != nil {
		log.Fatalf("Failed to open Requirements JSON file: %s", err)
	}
	defer reqsFile.Close()

	// Parse data
	var wg sync.WaitGroup
	wg.Add(2)

	var reqs pakr.Packages
	var idx []pakr.Dependency

	go func() {
		var err error
		reqs, err = ParseReqs(reqsFile)
		if err != nil {
			log.Fatalf("Failed to parse JSON from Requirements file: %s", err)
		}
		wg.Done()
	}()

	go func() {
		var err error
		idx, err = ParseIndex(idxFile)
		if err != nil {
			log.Fatalf("Failed to parse JSON from Index file: %s", err)
		}
		wg.Done()
	}()

	wg.Wait()

	resolver := pakr.NewResolver(reqs, idx)

	buf := bufio.NewWriter(os.Stdout)
	if err = WriteResults(buf, resolver); err != nil {
		log.Fatal(err.Error())
	}
	buf.Flush()
}

// A Packager that knows how to serialize to json
type Package struct {
	Prod string `json:"product"`
	Ver  string `json:"version"`
}

func (p Package) Version() string     { return p.Ver }
func (p Package) ProductName() string { return p.Prod }
func (p Package) PackageName() string { return fmt.Sprintf("%s-%s", p.Prod, p.Ver) }

// A Requirements type that knows how to serialize to json
type Requirements struct {
	Reqs []Package `json:"requires"`
}

// A Results type that knows how to serialize to json
type Results struct {
	Packages pakr.Packages `json:"results"`
	Solved   bool          `json:"solved"`
	Err      string        `json:"error"`
}

// A Dependency type that knows how to serialize to json
type Dependency struct {
	Target   Package     `json:"package"`
	Requires [][]Package `json:"requires"`
}

// An Index type that knows how to serialize to json
type Index struct {
	Deps []Dependency `json:"depends"`
}

// ParseReqs reads the json requirements file and parses
// it into a list of Packages
func ParseReqs(r io.Reader) (reqs pakr.Packages, err error) {
	var parsedReqs Requirements
	dec := json.NewDecoder(r)
	if err = dec.Decode(&parsedReqs); err != nil {
		return
	}

	// Convert parsed structure into a pakr structure
	reqs = make(pakr.Packages, 0, len(parsedReqs.Reqs))
	for _, parsedReq := range parsedReqs.Reqs {
		reqs = append(reqs, parsedReq)
	}
	return
}

// ParseIndex reads the json index file and parses
// it into an index, which is a list of available dependencies
func ParseIndex(r io.Reader) (deps []pakr.Dependency, err error) {
	var parsed Index
	dec := json.NewDecoder(r)
	if err = dec.Decode(&parsed); err != nil {
		return
	}

	// Convert the parsed structure into a pakr structure
	deps = make([]pakr.Dependency, 0, len(parsed.Deps))
	for _, parsedDep := range parsed.Deps {
		// Build each dependency
		dep := pakr.Dependency{parsedDep.Target, make([]pakr.Packages, 0, len(parsedDep.Requires))}
		for _, parsedPaks := range parsedDep.Requires {
			// Build each Package list
			paks := make(pakr.Packages, 0, len(parsedPaks))
			for _, parsedPak := range parsedPaks {
				paks = append(paks, parsedPak)
			}
			dep.Requires = append(dep.Requires, paks)
		}
		deps = append(deps, dep)
	}

	return
}

// WriteResults attempts to solve the Resolver and write the
// results to the io.Writer, in json format
func WriteResults(w io.Writer, resolver *pakr.Resolver) error {
	solved, err := resolver.Resolve()

	var errStr string
	if err != nil {
		fmt.Println("error!!!")
		errStr = err.Error()
	}

	res := Results{Packages: nil, Solved: solved, Err: errStr}

	if solved {
		res.Packages = resolver.Solution()

	} else {
		var buf bytes.Buffer
		fmt.Fprintln(&buf, "The following requirements cannot be satisfied:")
		for _, c := range resolver.Conflicts() {
			fmt.Fprintf(&buf, "    %s\n", c.PackageName())
		}

		fmt.Fprintln(&buf, "\nDetails:")
		detailed, _ := resolver.DetailedConflicts()
		fmt.Fprintln(&buf, detailed)

		res.Err = buf.String()
	}

	enc := json.NewEncoder(w)
	err = enc.Encode(&res)

	return err
}
