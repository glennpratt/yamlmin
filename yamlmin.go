package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"

	"github.com/glennpratt/yamlmin/pkg/yamlmin"
	"gopkg.in/yaml.v3"
)

func main() {
	minOccurrences := flag.Int("min-occurrences", 2, "Minimum number of occurrences to create anchor")
	minSize := flag.Int("min-size", 20, "Minimum structure size (chars) to consider for anchoring")
	indent := flag.Int("indent", 2, "Indentation level for output")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Finds and replaces duplicate YAML structures with anchors/aliases.\n")
		fmt.Fprintf(os.Stderr, "Reads from stdin and writes to stdout.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading stdin: %v\n", err)
		os.Exit(1)
	}

	if len(data) == 0 {
		return
	}

	opts := yamlmin.DefaultOptions()
	opts.MinOccurrences = *minOccurrences
	opts.MinSize = *minSize
	opts.Indent = *indent

	var val interface{}
	if err := yaml.Unmarshal(data, &val); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing YAML: %v\n", err)
		os.Exit(1)
	}

	out, err := yamlmin.MarshalWithOptions(val, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error processing YAML: %v\n", err)
		os.Exit(1)
	}

	// Count aliases (deduplicated references) in output via regex
	aliasRe := regexp.MustCompile(`\*(map|list|str)\d+`)
	aliases := aliasRe.FindAllString(string(out), -1)

	// Print stats to stderr
	fmt.Fprintf(os.Stderr, "Input: %d bytes, Output: %d bytes, Reduction: %.1f%%, Duplicates: %d\n",
		len(data), len(out), 100.0*(1.0-float64(len(out))/float64(len(data))), len(aliases))

	if _, err := os.Stdout.Write(out); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing stdout: %v\n", err)
		os.Exit(1)
	}
}
