package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

type counts struct{ statements, covered int64 }

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }
func run(args []string, out, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "usage: coveragegate coverage.out")
		return 2
	}
	f, err := os.Open(args[0])
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	defer f.Close()
	packages := map[string]counts{}
	total := counts{}
	scan := bufio.NewScanner(f)
	for scan.Scan() {
		fields := strings.Fields(scan.Text())
		if len(fields) != 3 {
			continue
		}
		file := strings.Split(fields[0], ":")[0]
		if strings.Contains(file, "/api/gen/") {
			continue
		}
		n, e := strconv.ParseInt(fields[1], 10, 64)
		if e != nil {
			fmt.Fprintln(stderr, e)
			return 2
		}
		hits, e := strconv.ParseInt(fields[2], 10, 64)
		if e != nil {
			fmt.Fprintln(stderr, e)
			return 2
		}
		pkg := file[:strings.LastIndex(file, "/")]
		c := packages[pkg]
		c.statements += n
		total.statements += n
		if hits > 0 {
			c.covered += n
			total.covered += n
		}
		packages[pkg] = c
	}
	if err := scan.Err(); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	failed := total.statements == 0
	fmt.Fprintf(out, "Overall (excluding generated api/gen only): %.2f%%\n", 100*float64(total.covered)/float64(total.statements))
	if total.covered*100 < total.statements*85 {
		failed = true
	}
	for pkg, c := range packages {
		core := false
		for _, name := range []string{"order", "product", "payment", "warehouse"} {
			if strings.Contains(pkg, "/internal/"+name+"/") {
				core = true
			}
		}
		if core {
			fmt.Fprintf(out, "%s: %.2f%%\n", pkg, 100*float64(c.covered)/float64(c.statements))
			if c.covered*100 < c.statements*85 {
				failed = true
			}
		}
	}
	if failed {
		fmt.Fprintln(stderr, "FAIL: coverage must reach 85% overall and in each core package")
		return 1
	}

	return 0
}
