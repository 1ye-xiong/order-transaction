package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type counts struct{ statements, covered int64 }

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: coveragegate coverage.out")
		os.Exit(2)
	}
	f, err := os.Open(os.Args[1])
	if err != nil {
		panic(err)
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
			panic(e)
		}
		hits, e := strconv.ParseInt(fields[2], 10, 64)
		if e != nil {
			panic(e)
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
		panic(err)
	}
	failed := total.statements == 0
	fmt.Printf("Overall (excluding generated api/gen only): %.2f%%\n", 100*float64(total.covered)/float64(total.statements))
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
			fmt.Printf("%s: %.2f%%\n", pkg, 100*float64(c.covered)/float64(c.statements))
			if c.covered*100 < c.statements*85 {
				failed = true
			}
		}
	}
	if failed {
		fmt.Fprintln(os.Stderr, "FAIL: coverage must reach 85% overall and in each core package")
		os.Exit(1)
	}
}
