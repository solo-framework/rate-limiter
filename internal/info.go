package internal

import (
	"fmt"
	"runtime/debug"
	"strings"
)

func DisplayBanner() {
	fmt.Println("  ____       _         _     _           _ _            ")
	fmt.Println(" |  _ \\ __ _| |_ ___  | |   (_)_ __ ___ (_) |_ ___ _ __ ")
	fmt.Println(" | |_) / _` | __/ _ \\ | |   | | '_ ` _ \\| | __/ _ \\ '__|")
	fmt.Println(" |  _ < (_| | ||  __/ | |___| | | | | | | | ||  __/ |   ")
	fmt.Println(" |_| \\_\\__,_|\\__\\___| |_____|_|_| |_| |_|_|\\__\\___|_|   ")
	fmt.Print(" (c) afi\n\n")

	fmt.Print(getInfo())
}

func getInfo() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return "no debug info\n"
	}

	out := new(strings.Builder)

	fmt.Fprintf(out, "\texec: %s\n\n", bi.Main.Path)
	out.WriteString("\tSettings:\n")

	for _, v := range bi.Settings {
		fmt.Fprintf(out, "\t %s: %s\n", v.Key, v.Value)
	}

	fmt.Fprintf(out, "\t go version: %s\n", bi.GoVersion)
	return out.String()
}
