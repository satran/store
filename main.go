package main

import (
	"bufio"
	"bytes"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

func main() {
	abs, err := filepath.Abs(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	store, err := walk(abs)
	if err != nil {
		log.Fatal(err)
	}
	http.HandleFunc("/", render(abs, store))
	http.Handle("/static/", http.StripPrefix("/static/",
		http.FileServer(http.Dir("static"))))
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}

func render(dir string, s map[string][]byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pageTmpl := template.Must(template.ParseFiles("templates/layout.html"))
		rd, ok := s[r.URL.Path]
		if !ok {
			name := filepath.Join(dir, r.URL.Path)
			http.ServeFile(w, r, name)
			return
		}
		err := pageTmpl.Execute(w, map[string]interface{}{
			"Name": r.URL.Path,
			"Page": template.HTML(rd),
		})
		if err != nil {
			log.Println("couldn't generate template ", err)
		}
	}
}

func walk(dir string) (map[string][]byte, error) {
	store := make(map[string][]byte)
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// prevent panic by handling failure accessing a path
			return err
		}
		if toParse(info.Name()) {
			f, err := os.Open(path)
			if err != nil {
				log.Fatal("open: ", err)
			}
			defer f.Close()
			log.Println("parsing ", path)
			r, err := parse(dir, f)
			if err != nil {
				log.Fatal("parse: ", err)
			}
			key := path[len(dir):]
			//TODO check err
			by, err := ioutil.ReadAll(r)
			if err != nil {
				return err
			}
			store[key] = by
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return store, nil
}

func toParse(name string) bool {
	return filepath.Ext(name) == ".txt"
}

var (
	macroReg = regexp.MustCompile(`^[\t ]*\#\|`)
	ismacro  = macroReg.MatchString

	taskReg = regexp.MustCompile(`^[\t ]*[\-\#] \[[ a-zA-Z]\] `)
	istask  = taskReg.MatchString
)

func parse(baseDir string, in io.Reader) (io.Reader, error) {
	r := bufio.NewReader(in)
	out := &bytes.Buffer{}
	lineNo := 1
	for {
		line, err := r.ReadString('\n')
		if err != nil && err != io.EOF {
			return nil, fmt.Errorf("ReadLine: %w", err)
		}
		if ismacro(line) {
			out.Write([]byte(fmt.Sprintf(`<div class="line" id="%d">`, lineNo)))
			if err := macro(removeMacroSyntax(line), baseDir, out); err != nil {
				return nil, err
			}
			out.Write([]byte("</div>"))
		} else if istask(line) {
			out.WriteString(taskAsHTML(line, lineNo))
		} else {
			out.WriteString(toHTML(line, lineNo))
		}
		lineNo++
		if err == io.EOF {
			break
		}
	}
	return out, nil
}

func taskAsHTML(line string, lineNo int) string {
	line = strings.Replace(line, "- [ ]", `<input class="task" type="checkbox" \>`, 1)
	line = strings.Replace(line, "- [x]", `<input class="task" checked="true" type="checkbox" \>`, 1)
	return toHTML(line, lineNo)
}

func toHTML(line string, lineNo int) string {
	if strings.HasPrefix(line, "# ") {
		return fmt.Sprintf(`<div class="line heading" id="%d">%s</div>`,
			lineNo, strings.TrimLeft(line, "# "))
	} else {
		return fmt.Sprintf(`<div class="line" id="%d">%s</div>`,
			lineNo, strings.TrimLeft(line, "# "))
	}
}

func removeMacroSyntax(line string) string {
	return macroReg.ReplaceAllString(line, "")
}

func macro(command string, dir string, out io.Writer) error {
	cmd := exec.Command("bash", "-c", command)
	cmd.Dir = dir
	cmd.Stdout = out
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command run: %w", err)
	}
	return nil
}
