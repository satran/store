package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"regexp"
)

func main() {
	f, err := os.Open(os.Args[1])
	if err != nil {
		log.Fatal("open: ", err)
	}
	defer f.Close()
	r, err := parse(f)
	if err != nil {
		log.Fatal("parse: ", err)
	}
	io.Copy(os.Stdout, r)
}

var macroReg = regexp.MustCompile(`^[\t ]*\#\|`)
var ismacro = macroReg.MatchString

func parse(in io.Reader) (io.Reader, error) {
	r := bufio.NewReader(in)
	out := &bytes.Buffer{}

	for {
		line, err := r.ReadString('\n')
		if err != nil && err != io.EOF {
			return nil, fmt.Errorf("ReadLine: %w", err)
		}
		if ismacro(line) {
			if err := macro(removeMacroSyntax(line), out); err != nil {
				return nil, err
			}
		} else {
			out.Write([]byte(line))
		}

		if err == io.EOF {
			break
		}
	}
	return out, nil
}

func removeMacroSyntax(line string) string {
	return macroReg.ReplaceAllString(line, "")
}

func macro(command string, out io.Writer) error {
	cmd := exec.Command("bash", "-c", command)
	cmd.Stdout = out
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command run: %w", err)
	}
	return nil
}
